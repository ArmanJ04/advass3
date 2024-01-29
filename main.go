package main

import (
	"database/sql"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"html/template"
	"log"
	"net/http"
	"strconv"
)

type Product struct {
	ID    int
	Name  string
	Price float64
}

var db *sql.DB

func main() {
	initDB()
	r := mux.NewRouter()
	r.HandleFunc("/", handleIndex)
	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func initDB() {
	var err error
	db, err = sql.Open("postgres", "user=postgres password=jansatov04 dbname=postgres sslmode=disable host=localhost port=3000")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255),
			price DECIMAL(10, 2)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
}
func handleIndex(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("filter")
	sort := r.URL.Query().Get("sort")
	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	query := "SELECT * FROM products"
	if filter != "" {
		query += fmt.Sprintf(" WHERE name LIKE '%%%s%%'", filter)
	}
	if sort != "" {
		query += fmt.Sprintf(" ORDER BY %s", sort)
	}
	limit := 10
	offset := (page - 1) * limit
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	var products []Product
	for rows.Next() {
		var product Product
		err := rows.Scan(&product.ID, &product.Name, &product.Price)
		if err != nil {
			log.Fatal(err)
		}
		products = append(products, product)
	}
	renderTemplate(w, products, filter, sort, page)
}

func renderTemplate(w http.ResponseWriter, products []Product, filter, sort string, page int) {
	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		log.Fatal(err)
	}
	data := struct {
		Products []Product
		Filter   string
		Sort     string
		Page     int
	}{
		Products: products,
		Filter:   filter,
		Sort:     sort,
		Page:     page,
	}
	err = tmpl.Execute(w, data)
	if err != nil {
		log.Fatal(err)
	}
}
