package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/Max20050/docuwave/internal/auth"
	"github.com/Max20050/docuwave/internal/datasource"
	"github.com/Max20050/docuwave/internal/llm"
	"github.com/Max20050/docuwave/internal/migrate"
	"github.com/Max20050/docuwave/internal/recipient"
	"github.com/Max20050/docuwave/internal/report"
	"github.com/Max20050/docuwave/internal/template"
)

func main() {
	_ = godotenv.Load("../../.env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	encryptionKey := os.Getenv("DATASOURCE_ENCRYPTION_KEY")
	if encryptionKey == "" {
		log.Fatal("DATASOURCE_ENCRYPTION_KEY is required")
	}

	llmEncryptionKey := os.Getenv("LLM_ENCRYPTION_KEY")
	if llmEncryptionKey == "" {
		log.Fatal("LLM_ENCRYPTION_KEY is required")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}
	log.Println("database connection established")

	if err := migrate.Run(context.Background(), pool); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrations applied")

	store := auth.NewStore(pool)
	tokenIssuer := auth.NewTokenIssuer(jwtSecret)
	authHandlers := auth.NewHandlers(store, tokenIssuer)

	googleConfig := auth.NewGoogleConfig(
		os.Getenv("GOOGLE_CLIENT_ID"),
		os.Getenv("GOOGLE_CLIENT_SECRET"),
		os.Getenv("GOOGLE_REDIRECT_URL"),
	)
	googleHandlers := auth.NewGoogleHandlers(store, tokenIssuer, googleConfig, frontendURL)

	encryptor, err := datasource.NewEncryptor(encryptionKey)
	if err != nil {
		log.Fatalf("invalid DATASOURCE_ENCRYPTION_KEY: %v", err)
	}
	dsStore := datasource.NewStore(pool)

	sheetsConfig := datasource.NewGoogleSheetsConfig(
		os.Getenv("GOOGLE_CLIENT_ID"),
		os.Getenv("GOOGLE_CLIENT_SECRET"),
		os.Getenv("GOOGLE_SHEETS_REDIRECT_URL"),
	)
	sheetsStore := datasource.NewSheetsStore(pool)
	// fieldMappings is shared by the resolver (a REST connector needs the
	// mapping to remap query results) and the field-mapping HTTP handlers.
	fieldMappings := datasource.NewFieldMappingStore(pool)
	resolver := datasource.NewResolver(dsStore, sheetsStore, encryptor, sheetsConfig, fieldMappings)
	// A source's structure is read when it's connected and kept, because that
	// stored picture is what report queries are built and checked against.
	schemas := datasource.NewSchemaProvider(resolver, datasource.NewSchemaStore(pool))
	schemaHandlers := datasource.NewSchemaHandlers(schemas, dsStore)
	fieldMappingHandlers := datasource.NewFieldMappingHandlers(fieldMappings, schemas)

	dsHandlers := datasource.NewHandlers(dsStore, encryptor, schemas)
	sheetsHandlers := datasource.NewSheetsHandlers(
		dsStore, sheetsStore, encryptor, sheetsConfig, tokenIssuer, schemas, frontendURL)
	restHandlers := datasource.NewRestHandlers(dsStore, encryptor, schemas)

	llmEncryptor, err := llm.NewEncryptor(llmEncryptionKey)
	if err != nil {
		log.Fatalf("invalid LLM_ENCRYPTION_KEY: %v", err)
	}
	llmStore := llm.NewStore(pool)
	llmHandlers := llm.NewHandlers(llmStore, llmEncryptor)
	// llmGenerator is also what a report's ai-summary blocks call, alongside
	// the natural-language-query path this was originally built for.
	llmGenerator := llm.NewGenerator(llmStore, llmEncryptor)
	// aiSummaryEnabled gates every ai-summary affordance off by default: the
	// product isn't meant to ship this to anyone until a cost estimator
	// exists to show what adding a block costs before they add it.
	aiSummaryEnabled := os.Getenv("AI_SUMMARY_ENABLED") == "true"

	reportStore := report.NewStore(pool)
	registry := template.NewRegistry(template.Starters()...)
	customTemplates := template.NewCustomStore(pool)
	templateArchive := template.NewArchiveStore(pool)
	// The composite source is the seam a per-user template — built in or
	// custom — plugs into: it merges the built-in starters with a user's own
	// saved designs, honoring what they've archived, into the one Source the
	// report pipeline depends on.
	templates := template.NewCompositeSource(registry, customTemplates, templateArchive)
	// The runner is the whole report pipeline — query, template, files — and is
	// what scheduled and on-demand delivery will run as well.
	reportRunner := report.NewRunner(resolver, schemas, templates, llmGenerator, fieldMappings)
	reportHandlers := report.NewHandlers(
		reportStore, reportRunner, templates, customTemplates, templateArchive, llmGenerator, aiSummaryEnabled)

	recipientStore := recipient.NewStore(pool)
	recipientGroups := recipient.NewGroupStore(pool)
	recipientHandlers := recipient.NewHandlers(recipientStore, recipientGroups)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("POST /api/auth/register", authHandlers.Register)
	mux.HandleFunc("POST /api/auth/login", authHandlers.Login)
	mux.HandleFunc("GET /api/me", authHandlers.RequireAuth(authHandlers.Me))
	mux.HandleFunc("GET /api/auth/google/login", googleHandlers.Login)
	mux.HandleFunc("GET /api/auth/google/callback", googleHandlers.Callback)
	mux.HandleFunc("GET /api/datasources", authHandlers.RequireAuth(dsHandlers.List))
	mux.HandleFunc("POST /api/datasources", authHandlers.RequireAuth(dsHandlers.Create))
	mux.HandleFunc("POST /api/datasources/test", authHandlers.RequireAuth(dsHandlers.TestConnection))
	mux.HandleFunc("DELETE /api/datasources/{id}", authHandlers.RequireAuth(dsHandlers.Delete))
	mux.HandleFunc("GET /api/datasources/{id}/schema", authHandlers.RequireAuth(schemaHandlers.Get))
	mux.HandleFunc("POST /api/datasources/{id}/schema/refresh", authHandlers.RequireAuth(schemaHandlers.Refresh))
	mux.HandleFunc("GET /api/datasources/{id}/field-mapping", authHandlers.RequireAuth(fieldMappingHandlers.Get))
	mux.HandleFunc("PUT /api/datasources/{id}/field-mapping", authHandlers.RequireAuth(fieldMappingHandlers.Put))
	mux.HandleFunc("GET /api/datasources/google-sheets/login", sheetsHandlers.Login)
	mux.HandleFunc("GET /api/datasources/google-sheets/callback", sheetsHandlers.Callback)
	mux.HandleFunc("GET /api/datasources/google-sheets/connections/{id}/spreadsheets", authHandlers.RequireAuth(sheetsHandlers.ListSpreadsheets))
	mux.HandleFunc("POST /api/datasources/google-sheets", authHandlers.RequireAuth(sheetsHandlers.Create))
	mux.HandleFunc("POST /api/datasources/rest-api/test", authHandlers.RequireAuth(restHandlers.TestConnection))
	mux.HandleFunc("POST /api/datasources/rest-api", authHandlers.RequireAuth(restHandlers.Create))
	mux.HandleFunc("GET /api/llm-config", authHandlers.RequireAuth(llmHandlers.Get))
	mux.HandleFunc("PUT /api/llm-config", authHandlers.RequireAuth(llmHandlers.Save))
	mux.HandleFunc("DELETE /api/llm-config", authHandlers.RequireAuth(llmHandlers.Delete))
	mux.HandleFunc("GET /api/report-templates", authHandlers.RequireAuth(reportHandlers.ListTemplates))
	mux.HandleFunc("GET /api/report-templates/archived", authHandlers.RequireAuth(reportHandlers.ListArchivedTemplates))
	mux.HandleFunc("POST /api/report-templates", authHandlers.RequireAuth(reportHandlers.CreateCustomTemplate))
	mux.HandleFunc("PUT /api/report-templates/{id}", authHandlers.RequireAuth(reportHandlers.UpdateCustomTemplate))
	mux.HandleFunc("POST /api/report-templates/{id}/archive", authHandlers.RequireAuth(reportHandlers.ArchiveTemplate))
	mux.HandleFunc("POST /api/report-templates/{id}/restore", authHandlers.RequireAuth(reportHandlers.RestoreTemplate))
	mux.HandleFunc("POST /api/reports/preview", authHandlers.RequireAuth(reportHandlers.Preview))
	mux.HandleFunc("POST /api/reports/preview-template", authHandlers.RequireAuth(reportHandlers.PreviewTemplate))
	mux.HandleFunc("POST /api/reports/preview-ai-summary", authHandlers.RequireAuth(reportHandlers.PreviewAISummary))
	mux.HandleFunc("GET /api/reports", authHandlers.RequireAuth(reportHandlers.List))
	mux.HandleFunc("POST /api/reports", authHandlers.RequireAuth(reportHandlers.Create))
	mux.HandleFunc("GET /api/reports/{id}/download", authHandlers.RequireAuth(reportHandlers.Download))
	mux.HandleFunc("DELETE /api/reports/{id}", authHandlers.RequireAuth(reportHandlers.Delete))
	mux.HandleFunc("GET /api/recipients", authHandlers.RequireAuth(recipientHandlers.List))
	mux.HandleFunc("POST /api/recipients", authHandlers.RequireAuth(recipientHandlers.Create))
	mux.HandleFunc("DELETE /api/recipients/{id}", authHandlers.RequireAuth(recipientHandlers.Delete))
	mux.HandleFunc("GET /api/recipient-groups", authHandlers.RequireAuth(recipientHandlers.ListGroups))
	mux.HandleFunc("POST /api/recipient-groups", authHandlers.RequireAuth(recipientHandlers.CreateGroup))
	mux.HandleFunc("DELETE /api/recipient-groups/{id}", authHandlers.RequireAuth(recipientHandlers.DeleteGroup))
	mux.HandleFunc("GET /api/recipient-groups/{id}/members", authHandlers.RequireAuth(recipientHandlers.ListMembers))
	mux.HandleFunc("POST /api/recipient-groups/{id}/members", authHandlers.RequireAuth(recipientHandlers.AddMember))
	mux.HandleFunc("DELETE /api/recipient-groups/{id}/members/{recipientId}", authHandlers.RequireAuth(recipientHandlers.RemoveMember))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func withCORS(next http.Handler) http.Handler {
	allowedOrigin := os.Getenv("FRONTEND_URL")
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:3000"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		// A downloaded report is fetched with the session's token and saved by
		// the browser, so the filename the server chose has to be readable.
		w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
