package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
)

func main() {
	// Configuración
	port := getEnv("PORT", "3000")
	jwtSecret := getEnv("GO_ANALYTICS_JWT_SECRET", "change_me_in_production")
	resolverToken := getEnv("SITE_RESOLVER_TOKEN", "change_me")

	// Conexión a Base de Datos
	dbHost := getEnv("POSTGRES_HOST", "postgres_analytics")
	dbPort := getEnv("POSTGRES_PORT", "5432")
	dbUser := getEnv("POSTGRES_USER", "analytics")
	dbPass := getEnv("POSTGRES_PASSWORD", "analytics")
	dbName := getEnv("POSTGRES_DB", "analytics")
	
	dbConnStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)
	
	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		log.Printf("Advertencia: No se pudo conectar a la base de datos: %v", err)
	} else {
		defer db.Close()
	}

	// Endpoints
	http.HandleFunc("/api/token", func(w http.ResponseWriter, r *http.Request) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"iss":           "main-backend",
			"aud":           "analytics-ingest",
			"site_code":     "pub_site_sandbox",
			"env":           "production",
			"token_version": 1,
			"iat":           time.Now().Unix(),
			"nbf":           time.Now().Unix(),
			"exp":           time.Now().Add(1 * time.Hour).Unix(),
			"jti":           fmt.Sprintf("sandbox_%d", time.Now().UnixNano()),
			"tenant_hint":   "tenant_sandbox",
			"site_hint":     "site_sandbox",
		})

		tokenString, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			http.Error(w, "Error generating token", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"token": tokenString,
		})
	})

	http.HandleFunc("/api/v1/internal/analytics/sites/resolve", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		expectedAuth := "Bearer " + resolverToken
		if authHeader != expectedAuth {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		
		// El worker es estricto y no soporta '*' como comodín, 
		// así que devolvemos el origin solicitado para que pase la validación.
		allowedDomains := []string{"*"}
		if origin, ok := req["origin"].(string); ok && origin != "" {
			allowedDomains = append(allowedDomains, origin)
		}

		log.Printf("Resolviendo site: %s para origin: %s -> permitiendo: %v", req["site_code"], req["origin"], allowedDomains)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"site_code":        req["site_code"],
			"tenant_id":        "tenant_sandbox",
			"site_id":          "site_sandbox",
			"status":           "active",
			"tracking_enabled": true,
			"allowed_domains":  allowedDomains,
			"token_version":    1,
			"sample_rate":      1,
			"schema_version":   1,
		})
	})

	http.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "Database not configured", http.StatusInternalServerError)
			return
		}

		rows, err := db.Query(`
			SELECT event_name, event_time, user_id, url, properties, context
			FROM analytics_events 
			ORDER BY event_time DESC 
			LIMIT 50
		`)
		if err != nil {
			log.Printf("Error querying events: %v", err)
			http.Error(w, "Error querying database", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var events []map[string]interface{}
		for rows.Next() {
			var name string
			var t time.Time
			var user sql.NullString
			var url sql.NullString
			var props string
			var ctx string

			if err := rows.Scan(&name, &t, &user, &url, &props, &ctx); err != nil {
				continue
			}

			var propsJSON, ctxJSON map[string]interface{}
			json.Unmarshal([]byte(props), &propsJSON)
			json.Unmarshal([]byte(ctx), &ctxJSON)

			event := map[string]interface{}{
				"event_name": name,
				"event_time": t,
				"user_id":    user.String,
				"url":        url.String,
				"properties": propsJSON,
				"context":    ctxJSON,
			}
			events = append(events, event)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(events)
	})

	// Servir archivos estáticos
	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/", fs)

	// Servir documentación (montada en /docs desde el host)
	docsFs := http.StripPrefix("/docs/", http.FileServer(http.Dir("/docs")))
	http.Handle("/docs/", docsFs)

	// Servir SDK (montado en /sdk desde el host)
	sdkFs := http.StripPrefix("/sdk/", http.FileServer(http.Dir("/sdk")))
	http.Handle("/sdk/", sdkFs)

	log.Printf("Sandbox Backend escuchando en el puerto %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
