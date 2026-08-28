// migrate-users imports the existing Firebase Auth users into Common ID.
// It deliberately defaults to dry-run; the real migration sends only the
// minimum identity fields required by Common ID and never sends passwords or
// Firebase ID tokens.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	firebaseauth "firebase.google.com/go/v4/auth"
	"github.com/joho/godotenv"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"gorm.io/gorm"
	"presentation-raffle/internal/infrastructure/database"
)

type inputUser struct {
	SourceUserID  string `json:"source_user_id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}
type migrationResult struct {
	SourceUserID string `json:"source_user_id"`
	Email        string `json:"email"`
	CommonUserID string `json:"common_user_id"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
}

func main() {
	// Load .env for local migration runs. Existing process environment values
	// remain authoritative because godotenv.Load does not overwrite them.
	_ = godotenv.Load()

	endpoint := flag.String("endpoint", env("COMMON_ID_API_ORIGIN", env("COMMON_ID_ORIGIN", "")), "Common ID backend API origin")
	apiKey := flag.String("api-key", env("COMMON_ID_API_KEY", ""), "Common ID application API key")
	clientID := flag.String("client-id", env("COMMON_ID_CLIENT_ID", ""), "Common ID client ID")
	projectID := flag.String("firebase-project-id", env("FIREBASE_PROJECT_ID", ""), "source Firebase project ID")
	credentialsJSON := flag.String("firebase-credentials", env("FIREBASE_SERVICE_ACCOUNT_JSON", ""), "Firebase service-account JSON")
	dryRun := flag.Bool("dry-run", true, "validate without creating Common ID users (default true)")
	updateDB := flag.Bool("update-db", false, "update user_models and raffle_models after Common ID migration")
	mapOutput := flag.String("mapping-output", "", "write source UID to Common ID UID mappings as JSONL")
	flag.Parse()

	for name, value := range map[string]string{"endpoint": *endpoint, "api-key": *apiKey, "client-id": *clientID, "firebase-project-id": *projectID} {
		if value == "" {
			if name == "api-key" {
				log.Fatal("-api-key is required; set COMMON_ID_API_KEY in .env or pass -api-key explicitly")
			}
			log.Fatalf("-%s is required", name)
		}
	}
	if !*dryRun && env("DATABASE_URL", "") == "" {
		log.Fatal("DATABASE_URL is required for a real migration")
	}
	if !*dryRun && !*updateDB {
		log.Fatal("-update-db=true is required for a real migration")
	}

	ctx := context.Background()
	httpClient := &http.Client{Timeout: 30 * time.Second}
	authClient := firebaseClient(ctx, *projectID, *credentialsJSON)
	users := collectUsers(ctx, authClient)
	if len(users) == 0 {
		log.Println("no Firebase users with email addresses found")
		return
	}

	var out io.Writer = os.Stdout
	var mapping io.WriteCloser
	if *mapOutput != "" {
		var err error
		mapping, err = os.Create(*mapOutput)
		if err != nil {
			log.Fatal(err)
		}
		defer mapping.Close()
		out = mapping
	}

	allResults := make([]migrationResult, 0, len(users))
	for start := 0; start < len(users); start += 1000 {
		end := start + 1000
		if end > len(users) {
			end = len(users)
		}
		results := migrateBatch(ctx, httpClient, *endpoint, *apiKey, *clientID, *dryRun, users[start:end])
		for _, result := range results {
			allResults = append(allResults, result)
			if result.Status == "failed" {
				log.Printf("migration failed for %s: %s", result.SourceUserID, result.Error)
			}
		}
	}
	if !*dryRun && *updateDB {
		db, err := database.NewPostgresConnection(env("DATABASE_URL", ""))
		if err != nil {
			log.Fatal(err)
		}
		if err := updateApplicationUsers(ctx, db, allResults); err != nil {
			log.Fatalf("update application database: %v", err)
		}
	}
	for _, result := range allResults {
		if err := json.NewEncoder(out).Encode(result); err != nil {
			log.Fatal(err)
		}
	}
	if *dryRun {
		log.Println("dry-run completed; rerun with -dry-run=false after reviewing the output")
	}
}

func updateApplicationUsers(ctx context.Context, db *gorm.DB, results []migrationResult) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, result := range results {
			if result.CommonUserID == "" || result.Status == "dry_run" || result.Status == "failed" || result.SourceUserID == result.CommonUserID {
				continue
			}
			var oldCount, newCount int64
			if err := tx.Table("user_models").Where("uid = ?", result.SourceUserID).Count(&oldCount).Error; err != nil {
				return err
			}
			if err := tx.Table("user_models").Where("uid = ?", result.CommonUserID).Count(&newCount).Error; err != nil {
				return err
			}
			if oldCount == 0 && newCount > 0 {
				continue
			}
			if oldCount == 0 {
				return fmt.Errorf("old user %s was not found", result.SourceUserID)
			}
			if newCount > 0 {
				return fmt.Errorf("new UID %s already exists", result.CommonUserID)
			}
			insert := `INSERT INTO user_models (created_at, updated_at, deleted_at, uid, email, display_name, photo_url, provider, email_verified, last_login_at)
SELECT created_at, ?, deleted_at, ?, COALESCE(NULLIF(?, ''), email), display_name, photo_url, 'common-id', email_verified, last_login_at
FROM user_models WHERE uid = ?`
			if err := tx.Exec(insert, time.Now().UTC(), result.CommonUserID, result.Email, result.SourceUserID).Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE raffle_models SET user_uid = ? WHERE user_uid = ?", result.CommonUserID, result.SourceUserID).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM user_models WHERE uid = ?", result.SourceUserID).Error; err != nil {
				return err
			}
			log.Printf("updated application user %s -> %s", result.SourceUserID, result.CommonUserID)
		}
		return nil
	})
}

func firebaseClient(ctx context.Context, project, raw string) *firebaseauth.Client {
	var opts []option.ClientOption
	if raw == "" {
		log.Fatal("FIREBASE_SERVICE_ACCOUNT_JSON must contain service-account JSON")
	}
	opts = append(opts, option.WithCredentialsJSON([]byte(raw)))
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: project}, opts...)
	if err != nil {
		log.Fatalf("initialize Firebase: %v", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("initialize Firebase Auth: %v", err)
	}
	return client
}

func collectUsers(ctx context.Context, client *firebaseauth.Client) []inputUser {
	it := client.Users(ctx, "")
	users := make([]inputUser, 0)
	for {
		user, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("list Firebase users: %v", err)
		}
		if user.Email == "" {
			log.Printf("skip Firebase UID %s: no email", user.UID)
			continue
		}
		users = append(users, inputUser{SourceUserID: user.UID, Email: strings.TrimSpace(user.Email), EmailVerified: user.EmailVerified})
	}
	return users
}

func migrateBatch(ctx context.Context, httpClient *http.Client, endpoint, apiKey, clientID string, dryRun bool, users []inputUser) []migrationResult {
	body, err := json.Marshal(map[string]any{"client_id": clientID, "dry_run": dryRun, "users": users})
	if err != nil {
		log.Fatal(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/v1/migrations/users", strings.NewReader(string(body)))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)
	res, err := httpClient.Do(req)
	if err != nil {
		log.Fatalf("Common ID migration request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		response, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		log.Fatalf("Common ID migration failed: HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(response)))
	}
	var payload struct {
		Results []migrationResult `json:"results"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		log.Fatal(fmt.Errorf("decode migration response: %w", err))
	}
	return payload.Results
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
