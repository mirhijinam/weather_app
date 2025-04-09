package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type Forecast struct {
	City      string    `json:"city"`
	UpdatedAt time.Time `json:"updated_at"`
	Forecast  string    `json:"forecast"`
}

var db *sql.DB

func initDB() {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbPort == "" {
		dbPort = "5432"
	}
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPass, dbName)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error connecting to DB: %v", err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS weather (
		city TEXT PRIMARY KEY,
		updated_at TIMESTAMP,
		forecast TEXT
	);
	`
	if _, err := db.Exec(createTable); err != nil {
		log.Fatalf("Error creating table: %v", err)
	}
}

func getForecastFromDB(city string) (*Forecast, error) {
	query := `SELECT city, updated_at, forecast FROM weather WHERE city = $1`
	row := db.QueryRow(query, city)
	var fc Forecast
	err := row.Scan(&fc.City, &fc.UpdatedAt, &fc.Forecast)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &fc, nil
}

func updateForecastInDB(f Forecast) error {
	query := `
		INSERT INTO weather (city, updated_at, forecast)
		VALUES ($1, $2, $3)
		ON CONFLICT (city)
		DO UPDATE SET updated_at = EXCLUDED.updated_at, forecast = EXCLUDED.forecast
	`
	_, err := db.Exec(query, f.City, f.UpdatedAt, f.Forecast)
	return err
}

func fetchForecastFromAPI(city string) (string, error) {
	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		err := fmt.Errorf("WEATHER_API_KEY variable is not set")
		log.Println("ERROR:", err)
		return "", err
	}

	apiURL := fmt.Sprintf("https://api.weatherapi.com/v1/current.json?key=%s&q=%s&aqi=no", apiKey, city)

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error from weather API: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var data struct {
		Current struct {
			TempC float64 `json:"temp_c"`
		} `json:"current"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}

	forecast := fmt.Sprintf("%.1f", data.Current.TempC)
	return forecast, nil
}

func forecastHandler(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city_name")
	if city == "" {
		http.Error(w, "city_name parameter is required", http.StatusBadRequest)
		return
	}

	forecastRecord, err := getForecastFromDB(city)
	if err != nil {
		http.Error(w, "error querying DB", http.StatusInternalServerError)
		return
	}

	if forecastRecord != nil && time.Since(forecastRecord.UpdatedAt) < 30*time.Minute {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"city":       city,
			"forecast":   forecastRecord.Forecast,
			"updated_at": forecastRecord.UpdatedAt,
		})
		return
	}

	forecastData, err := fetchForecastFromAPI(city)
	if err != nil {
		http.Error(w, "error fetching data from API", http.StatusInternalServerError)
		return
	}

	newForecast := Forecast{
		City:      city,
		UpdatedAt: time.Now(),
		Forecast:  forecastData,
	}

	if err := updateForecastInDB(newForecast); err != nil {
		http.Error(w, "error updating DB", http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"city":       city,
		"forecast":   forecastData,
		"updated_at": newForecast.UpdatedAt,
	})
}

func main() {
	initDB()
	defer db.Close()

	http.HandleFunc("/forecast", forecastHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "7070"
	}
	log.Printf("Server on port :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
