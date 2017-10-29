package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// CelsiusToKelvin for convert from Celsius to Kelvin.
const CelsiusToKelvin = 273.15

type weatherProvider interface {
	temperature(city string) (float64, error)
}

type multiWeatherProvider []weatherProvider

type openWeatherMap struct {
	apiKey string
}

type weatherUnderground struct {
	apiKey string
}

func main() {
	http.HandleFunc("/hello", hello)
	http.HandleFunc("/weather/", getWeather)

	http.ListenAndServe(":8080", nil)
}

func hello(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello"))
}

func getWeather(w http.ResponseWriter, r *http.Request) {
	mw := multiWeatherProvider{
		openWeatherMap{
			apiKey: os.Getenv("API_KEY_OpenWeatherMap"),
		},
		weatherUnderground{
			apiKey: os.Getenv("API_KEY_WeatherUnderground"),
		},
	}

	begin := time.Now()
	city := strings.SplitN(r.URL.Path, "/", 10)[2]
	fmt.Println(strings.SplitN(r.URL.Path, "/", 10))

	temp, err := mw.temperature(city)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//data.Main.Temp -= K_TO_C
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"city": city,
		"temp": temp,
		"took": time.Since(begin).String(),
	})
}

func (w openWeatherMap) temperature(city string) (float64, error) {
	resp, err := http.Get("http://api.openweathermap.org/data/2.5/weather?APPID=" + w.apiKey + "&q=" + city)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	var d struct {
		Main struct {
			Temp float64 `json:"temp"`
		} `json:"main"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}

	log.Printf("openWeatherMap: %s: %.2f", city, d.Main.Temp)
	return d.Main.Temp, nil
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

	kelvin := d.Observation.Celsius + CelsiusToKelvin
	log.Printf("weatherUnderground: %s: %.2f", city, kelvin)

	return kelvin, nil
}

func (w multiWeatherProvider) temperature(city string) (float64, error) {
	sum := 0.0

	for _, provider := range w {
		k, err := provider.temperature(city)
		if err != nil {
			return 0, err
		}

		sum += k
	}

	return sum / float64(len(w)), nil
}
