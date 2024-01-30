package main

import (
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"html/template"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Product struct {
	gorm.Model
	Name  string
	Price float64
}

var db *gorm.DB
var log *logrus.Logger

func main() {
	initLogger()
	initDB()

	r := mux.NewRouter()
	lim := tollbooth.NewLimiter(1, &limiter.ExpirableOptions{DefaultExpirationTTL: 5 * time.Second})
	r.Handle("/", tollbooth.LimitHandler(lim, http.HandlerFunc(handleIndex)))
	r.Handle("/register", tollbooth.LimitHandler(lim, http.HandlerFunc(handleRegister))).Methods("POST")
	r.Handle("/update", tollbooth.LimitHandler(lim, http.HandlerFunc(handleUpdate))).Methods("POST")
	r.Handle("/delete", tollbooth.LimitHandler(lim, http.HandlerFunc(handleDelete))).Methods("POST")

	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func initLogger() {
	log = logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	file, err := os.OpenFile("logfile.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(file)
	} else {
		log.Error("Failed to open log file:", err)
	}
	log.SetOutput(io.MultiWriter(os.Stdout, file))
}

func initDB() {
	var err error
	dsn := "user=postgres password=jansatov04 dbname=postgres sslmode=disable host=localhost port=3000"
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Error initializing database:", err)
	}
	db.AutoMigrate(&Product{})
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	var products []Product

	filter := r.URL.Query().Get("filter")
	sort := r.URL.Query().Get("sort")
	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	query := db
	if filter != "" {
		query = query.Where("name LIKE ?", "%"+filter+"%")
	}

	switch {
	case strings.HasPrefix(sort, "name"):
		query = query.Order("name " + strings.TrimPrefix(sort, "name_"))
	case strings.HasPrefix(sort, "price"):
		query = query.Order("price " + strings.TrimPrefix(sort, "price_"))
	}

	limit := 10
	offset := (page - 1) * limit
	query = query.Limit(limit).Offset(offset)

	query.Find(&products)

	renderTemplate(w, products, filter, sort, page, "", "")
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	priceStr := r.FormValue("price")

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		renderTemplate(w, nil, "", "", 1, "", "Invalid price")
		log.WithFields(logrus.Fields{
			"action": "register",
			"status": "failure",
			"error":  "Invalid price",
		}).Error("Failed to register product")
		return
	}

	newProduct := Product{Name: name, Price: price}
	db.Create(&newProduct)

	http.Redirect(w, r, "/", http.StatusSeeOther)
	log.WithFields(logrus.Fields{
		"action":  "register",
		"status":  "success",
		"product": newProduct,
	}).Info("Product registered successfully")
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	productIDStr := r.FormValue("productIdUpdate")
	newName := r.FormValue("newName")
	newPriceStr := r.FormValue("newPrice")

	productID, err := strconv.ParseUint(productIDStr, 10, 64)
	if err != nil {
		renderTemplate(w, nil, "", "", 1, "", "Invalid Product ID")
		log.WithFields(logrus.Fields{
			"action": "update",
			"status": "failure",
			"error":  "Invalid Product ID",
		}).Error("Failed to update product")
		return
	}

	var updateProduct Product
	result := db.First(&updateProduct, productID)
	if result.Error != nil {
		renderTemplate(w, nil, "", "", 1, "", "Product not found")
		log.WithFields(logrus.Fields{
			"action": "update",
			"status": "failure",
			"error":  "Product not found",
		}).Error("Failed to update product")
		return
	}

	if newName != "" {
		updateProduct.Name = newName
	}
	if newPriceStr != "" {
		newPrice, err := strconv.ParseFloat(newPriceStr, 64)
		if err != nil {
			renderTemplate(w, nil, "", "", 1, "", "Invalid price")
			log.WithFields(logrus.Fields{
				"action": "update",
				"status": "failure",
				"error":  "Invalid price",
			}).Error("Failed to update product")
			return
		}
		updateProduct.Price = newPrice
	}
	db.Save(&updateProduct)

	http.Redirect(w, r, "/", http.StatusSeeOther)
	log.WithFields(logrus.Fields{
		"action":  "update",
		"status":  "success",
		"product": updateProduct,
	}).Info("Product updated successfully")
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	productIDStr := r.FormValue("productIdDelete")

	productID, err := strconv.ParseUint(productIDStr, 10, 64)
	if err != nil {
		renderTemplate(w, nil, "", "", 1, "", "Invalid Product ID")
		log.WithFields(logrus.Fields{
			"action": "delete",
			"status": "failure",
			"error":  "Invalid Product ID",
		}).Error("Failed to delete product")
		return
	}

	var deleteProduct Product
	result := db.First(&deleteProduct, productID)
	if result.Error != nil {
		renderTemplate(w, nil, "", "", 1, "", "Product not found")
		log.WithFields(logrus.Fields{
			"action": "delete",
			"status": "failure",
			"error":  "Product not found",
		}).Error("Failed to delete product")
		return
	}

	db.Delete(&deleteProduct, productID)

	http.Redirect(w, r, "/", http.StatusSeeOther)
	log.WithFields(logrus.Fields{
		"action":  "delete",
		"status":  "success",
		"product": deleteProduct,
	}).Info("Product deleted successfully")
}

func renderTemplate(w http.ResponseWriter, products []Product, filter, sort string, page int, successMsg, errorMsg string) {
	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		log.Fatal(err)
	}

	data := struct {
		Products   []Product
		Filter     string
		Sort       string
		Page       int
		SuccessMsg string
		ErrorMsg   string
	}{
		Products:   products,
		Filter:     filter,
		Sort:       sort,
		Page:       page,
		SuccessMsg: successMsg,
		ErrorMsg:   errorMsg,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Fatal(err)
	}
}
