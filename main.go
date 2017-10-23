package main

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	_ "github.com/lib/pq"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

type Book struct {
	PK             int
	Title          string
	Author         string
	Classification string
}

type SearchResult struct {
	Title  string `xml:"title,attr"`
	Author string `xml:"author,attr"`
	Year   string `xml:"hyr,attr"`
	ID     string `xml:"owi,attr"`
}

type Page struct {
	Books []Book
}

const (
	host     = "elmer.db.elephantsql.com"
	port     = 5432
	user     = "htldhvag"
	password = ""
	dbname   = "htldhvag"
)

func main() {

	templates := template.Must(template.ParseFiles("templates/index.html"))

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	fmt.Println(psqlInfo)

	db, err := sql.Open("postgres", psqlInfo)

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := Page{Books: []Book{}}

		rows, err := db.Query("SELECT pk, title, author, classification FROM books")

		if err != nil {
			fmt.Printf("ERROR IS: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		defer rows.Close()
		for rows.Next() {
			var b Book
			rows.Scan(&b.PK, &b.Title, &b.Author, &b.Classification)
			p.Books = append(p.Books, b)
		}

		if err := templates.ExecuteTemplate(w, "index.html", p); err != nil {
			fmt.Printf("ERROR IS: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		var results []SearchResult
		var err error

		if results, err = search(r.FormValue("search")); err != nil {
			fmt.Printf("ERROR IS: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		encoder := json.NewEncoder(w)
		if err := encoder.Encode(results); err != nil {
			fmt.Printf("ERROR IS: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/books/add", func(w http.ResponseWriter, r *http.Request) {
		var book ClassifyBookResponse
		var err error

		if book, err = find(r.FormValue("id")); err != nil {
			fmt.Printf("ERROR IS: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err = db.Ping(); err != nil {
			fmt.Printf("ERROR IS: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		result, err := db.Exec("INSERT INTO books(title, author, id, classification) values($1, $2, $3, $4)",
			book.BookData.Title, book.BookData.Author, book.BookData.ID, book.Classification.MostPopular)

		if err != nil {
			fmt.Printf("ERROR IS: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		pk, _ := result.LastInsertId()

		b := Book{
			PK:             int(pk),
			Title:          book.BookData.Title,
			Author:         book.BookData.Author,
			Classification: book.Classification.MostPopular,
		}

		if err := json.NewEncoder(w).Encode(b); err != nil {
			fmt.Printf("ERROR IS: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/books/delete", func(w http.ResponseWriter, r *http.Request) {
		if _, err := db.Exec("DELETE FROM books WHERE books.pk = $1", r.FormValue("pk")); err != nil {
			fmt.Printf("ERROR IS: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	fmt.Println(http.ListenAndServe(":8080", nil))
}

type ClassifySearchResponse struct {
	Results []SearchResult `xml:"works>work"`
}

type ClassifyBookResponse struct {
	BookData struct {
		Title  string `xml:"title,attr"`
		Author string `xml:"author,attr"`
		ID     string `xml:"owi,attr"`
	} `xml:"work"`
	Classification struct {
		MostPopular string `xml:"sfa,attr"`
	} `xml:"recommendations>ddc>mostPopular"`
}

func find(id string) (ClassifyBookResponse, error) {
	var c ClassifyBookResponse
	body, err := classifyAPI("http://classify.oclc.org/classify2/Classify?summary=true&owi=" + url.QueryEscape(id))

	if err != nil {
		fmt.Printf("ERROR IS: %s", err)
		return ClassifyBookResponse{}, err
	}

	err = xml.Unmarshal(body, &c)
	return c, err
}

func search(query string) ([]SearchResult, error) {
	var c ClassifySearchResponse
	body, err := classifyAPI("http://classify.oclc.org/classify2/Classify?summary=true&title=" + url.QueryEscape(query))

	if err != nil {
		fmt.Printf("ERROR IS: %s", err)
		return []SearchResult{}, err
	}

	err = xml.Unmarshal(body, &c)
	return c.Results, err
}

func classifyAPI(url string) ([]byte, error) {
	var resp *http.Response
	var err error

	if resp, err = http.Get(url); err != nil {
		fmt.Printf("ERROR IS: %s", err)
		return []byte{}, err
	}

	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
