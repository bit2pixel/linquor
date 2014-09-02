package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

func main() {

	keyStr, err := ioutil.ReadFile("key.txt")
	if err != nil {
		panic("Cannot find key.txt")
	}
	key := strings.Split(string(keyStr), "\n")[0]

	fmt.Println("Key -> ", key)

	wundergroundAPIKey := flag.String("wunderground.api.key", key, "wunderground.com API key")
	flag.Parse()

	mw := multiWeatherProvider{
		openWeatherMap{},
		weatherUnderground{apiKey: *wundergroundAPIKey},
	}

	http.HandleFunc("/", indexGetHandler)
	http.HandleFunc("/weather/", func(w http.ResponseWriter, r *http.Request) {
		weatherGetHandler(mw, w, r)
	})

	http.ListenAndServe(":8000", nil)
}

func indexGetHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello Kitty!"))
}

func weatherGetHandler(mw multiWeatherProvider, w http.ResponseWriter, r *http.Request) {
	begin := time.Now()
	city := strings.SplitN(r.URL.Path, "/", 3)[2]

	temp, err := mw.temperature(city)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"city": city,
		"temp": temp,
		"took": time.Since(begin).String(),
	})
}

type weatherData struct {
	Main struct {
		Kelvin float64 `json:"temp"`
	} `json:"main"`
}

type weatherProvider interface {
	temperature(city string) (float64, error) // in Kelvin
}

type openWeatherMap struct{}

func (w openWeatherMap) temperature(city string) (float64, error) {
	resp, err := http.Get("http://api.openweathermap.org/data/2.5/weather?q=" + city)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	var d weatherData

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}

	log.Printf("openWeatherMap: %s: %.2f", city, d.Main.Kelvin)

	return d.Main.Kelvin, nil
}

type weatherUnderground struct {
	apiKey string
}

func (w weatherUnderground) temperature(city string) (float64, error) {
	resp, err := http.Get("http://api.wunderground.com/api/" + w.apiKey + "/conditions/q/" + city + ".json")
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	var d struct {
		Observation struct {
			Celsius float64 `json:"temp_c"`
		} `json:"current_observation"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}

	kelvin := d.Observation.Celsius + 273.15
	log.Printf("weatherUnderground: %s: %.2f", city, kelvin)
	return kelvin, nil
}

func temperature(city string, providers ...weatherProvider) (float64, error) {
	sum := 0.0

	for _, provider := range providers {
		k, err := provider.temperature(city)
		if err != nil {
			return 0, err
		}

		sum += k
	}

	return sum / float64(len(providers)), nil
}

type multiWeatherProvider []weatherProvider

func (w multiWeatherProvider) temperature(city string) (float64, error) {
	temps := make(chan float64, len(w))
	errs := make(chan error, len(w))

	for _, provider := range w {
		go func(p weatherProvider) {
			k, err := p.temperature(city)
			if err != nil {
				errs <- err
				return
			}
			temps <- k
		}(provider)
	}

	sum := 0.0

	for i := 0; i < len(w); i++ {
		select {
		case temp := <-temps:
			sum += temp
		case err := <-errs:
			return 0, err
		}
	}

	return sum / float64(len(w)), nil
}
