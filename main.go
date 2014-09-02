package main

import "net/http"

func main() {
	http.HandleFunc("/", index)
	http.ListenAndServe(":8000", nil)
	println("hello kitty!")
}

func index(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello Kitty!"))
}
