package main

import (
	"encoding/json"
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
	log.Println(strings.SplitN(r.URL.Path, "/", 10))

	celsius, err := mw.temperature(city)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//data.Main.Temp -= K_TO_C
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"city":    city,
		"celsius": celsius,
		"took":    time.Since(begin).String(),
	})
}

func (w openWeatherMap) temperature(city string) (float64, error) {
	//log.Print("openWeatherMap=", w.apiKey)
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

	temp := d.Main.Temp - CelsiusToKelvin
	log.Printf("openWeatherMap: %s: %.2f", city, temp)
	return temp, nil
}

func (w weatherUnderground) temperature(city string) (float64, error) {
	//log.Print("weatherUnderground", w.apiKey)
	resp, err := http.Get("http://api.wunderground.com/api/" + w.apiKey + "/conditions/q/Japan/" + city + ".json")
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

	temp := d.Observation.Celsius
	log.Printf("weatherUnderground: %s: %.2f", city, temp)

	return temp, nil
}

func (w multiWeatherProvider) temperature(city string) (float64, error) {
	// Make a channel for temperatures, and a channel for errors.
	// Each provider will push a value into only one.
	temps := make(chan float64, len(w))
	errs := make(chan error, len(w))

	// For each provider, spawn a goroutine with an anonymous function.
	// That function will invoke the temperature method, and forward the response.
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

	// Collect a temperature or an error from each provider.
	for i := 0; i < len(w); i++ {
		select {
		case temp := <-temps:
			sum += temp
		case err := <-errs:
			return 0, err
		}
	}

	// Return the average, same as before.
	return sum / float64(len(w)), nil
}
