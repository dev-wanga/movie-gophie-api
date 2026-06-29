package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	JWT_TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1aWQiOjY3NTU3ODExOTM4NTkyOTYwOCwiYXRwIjozLCJleHQiOiIxNzgyNjY3MjA1IiwiZXhwIjoxNzkwNDQzMjA1LCJpYXQiOjE3ODI2NjY5MDV9.2n0yqU3dhrbgyLAc6EonU6q3gd28HJsKDetkM69N0uc"
	H5_API    = "https://h5-api.aoneroom.com"
	SPORT_API = "https://h5-sport-api.aoneroom.com"
	NETFILM   = "https://netfilm.world"
	UA        = "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36"
	UUID      = "d8c3539e-2e46-4000-af20-7046a856e30a"
	client    = &http.Client{Timeout: 25 * time.Second}
)

func main() {
	port := os.Getenv("PORT")
	if port == "" { port = "3001" }

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/search", handleSearch)
	http.HandleFunc("/stream", handleStream)
	http.HandleFunc("/health", handleHealth)
	// Sports
	http.HandleFunc("/sports", handleSports)
	http.HandleFunc("/sports/fifa", handleFIFA)
	http.HandleFunc("/sports/wwe", handleWWE)
	http.HandleFunc("/sports/live", handleLive)
	http.HandleFunc("/sports/stream", handleSportStream)
	http.HandleFunc("/watch", handleWatch)
	http.HandleFunc("/download", handleDownload)

	log.Printf("Movie+Sports API on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	cors(w)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"api": "Movie+Sports API v1.0",
		"sources": []string{"netfilm.world", "h5-api", "h5-sport-api", "2embed.cc"},
		"endpoints": []string{
			"/search?q=avengers", "/stream?id=&path=",
			"/sports", "/sports/fifa", "/sports/wwe", "/sports/live", "/sports/stream?id=", "/watch?id=&path=", "/download?id=&path=",
		},
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	cors(w)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	cors(w)
	q := r.URL.Query().Get("q")
	if q == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "q required"})
		return
	}
	var allResults []interface{}
	if data := searchH5API(q); data != nil {
		allResults = append(allResults, data)
	}
	if data := search2Embed(q); data != nil {
		allResults = append(allResults, data)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"query": q, "count": len(allResults), "results": allResults,
	})
}

func handleStream(w http.ResponseWriter, r *http.Request) {
	cors(w)
	id := r.URL.Query().Get("id")
	path := r.URL.Query().Get("path")
	se := r.URL.Query().Get("se")
	ep := r.URL.Query().Get("ep")
	if id == "" || path == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "id and path required"})
		return
	}
	if se == "" { se = "0" }
	if ep == "" { ep = "0" }

	url := fmt.Sprintf("%s/wefeed-h5api-bff/subject/play?subjectId=%s&se=%s&ep=%s&detailPath=%s", NETFILM, id, se, ep, path)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", UA)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", fmt.Sprintf("%s/spa/videoPlayPage/movies/%s", NETFILM, path))
	req.Header.Set("Origin", NETFILM)
	req.Header.Set("Cookie", fmt.Sprintf("uuid=%s; token=%s", UUID, JWT_TOKEN))

	resp, err := client.Do(req)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var data map[string]interface{}
	json.Unmarshal(body, &data)

	if streams, ok := data["data"].(map[string]interface{})["streams"].([]interface{}); ok && len(streams) > 0 {
		var sources []map[string]interface{}
		for _, s := range streams {
			stream := s.(map[string]interface{})
			sources = append(sources, map[string]interface{}{
				"quality": fmt.Sprintf("%sp", stream["resolutions"]),
				"format":  stream["format"],
				"url":     stream["url"],
			})
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true, "id": id, "count": len(sources), "sources": sources,
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false, "error": "No streams available",
		})
	}
}

// ═══ SPORTS ═══

func sportFetch(path string) (map[string]interface{}, error) {
	resp, err := client.Get(SPORT_API + path)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	if code, _ := data["code"].(float64); code == 0 {
		if d, ok := data["data"].(map[string]interface{}); ok { return d, nil }
	}
	return data, nil
}

func handleSports(w http.ResponseWriter, r *http.Request) {
	cors(w)
	data, err := sportFetch("/wefeed-h5api-bff/home?host=sportslivetoday.com")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}
	var sections []map[string]interface{}
	if ops, ok := data["operatingList"].([]interface{}); ok {
		for _, op := range ops {
			o := op.(map[string]interface{})
			subs := o["subjects"].([]interface{})
			var subjects []map[string]interface{}
			for _, s := range subs {
				sub := s.(map[string]interface{})
				subjects = append(subjects, map[string]interface{}{
					"id": sub["subjectId"], "title": sub["title"],
				})
			}
			sections = append(sections, map[string]interface{}{
				"title": o["title"], "type": o["type"],
				"count": len(subjects), "subjects": subjects,
			})
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "sections": sections})
}

func handleFIFA(w http.ResponseWriter, r *http.Request) {
	cors(w)
	data, _ := sportFetch("/wefeed-h5api-bff/home?host=sportslivetoday.com")
	var fifa map[string]interface{}
	if ops, ok := data["operatingList"].([]interface{}); ok {
		for _, op := range ops {
			o := op.(map[string]interface{})
			t := strings.ToLower(fmt.Sprint(o["title"]))
			if strings.Contains(t, "fifa") || strings.Contains(t, "world cup") {
				fifa = o
				break
			}
		}
	}
	if fifa == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "FIFA section not found"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true, "title": fifa["title"],
		"liveList": fifa["liveList"],
	})
}

func handleWWE(w http.ResponseWriter, r *http.Request) {
	cors(w)
	data, _ := sportFetch("/wefeed-h5api-bff/home?host=sportslivetoday.com")
	var wwe map[string]interface{}
	if ops, ok := data["operatingList"].([]interface{}); ok {
		for _, op := range ops {
			o := op.(map[string]interface{})
			t := strings.ToLower(fmt.Sprint(o["title"]))
			if strings.Contains(t, "wwe") || strings.Contains(t, "wrestling") {
				wwe = o
				break
			}
		}
	}
	if wwe == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "WWE section not found"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true, "title": wwe["title"],
		"subjects": wwe["subjects"],
	})
}

func handleLive(w http.ResponseWriter, r *http.Request) {
	cors(w)
	data, _ := sportFetch("/wefeed-h5api-bff/home?host=sportslivetoday.com")
	var live map[string]interface{}
	if ops, ok := data["operatingList"].([]interface{}); ok {
		for _, op := range ops {
			o := op.(map[string]interface{})
			if fmt.Sprint(o["type"]) == "SPORT_LIVE" {
				live = o
				break
			}
		}
	}
	if live == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "No live sports"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true, "title": live["title"],
		"liveNow": live["liveList"], "upcoming": live["subjects"],
	})
}

func handleSportStream(w http.ResponseWriter, r *http.Request) {
	cors(w)
	id := r.URL.Query().Get("id")
	sportType := r.URL.Query().Get("sportType")
	if id == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "id required"})
		return
	}
	if sportType == "" { sportType = "football" }

	data, _ := sportFetch(fmt.Sprintf("/wefeed-h5api-bff/subject/play?subjectId=%s&sportType=%s", id, sportType))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true, "id": id,
		"streams": data["streams"], "hasResource": data["hasResource"],
	})
}

// ═══ HELPERS ═══

func cors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
}

func searchH5API(q string) interface{} {
	body := fmt.Sprintf(`{"keyword":"%s","perPage":10,"page":1}`, q)
	resp, err := client.Post(H5_API+"/wefeed-h5api-bff/subject/search", "application/json", strings.NewReader(body))
	if err != nil { return nil }
	defer resp.Body.Close()

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	if data["data"] == nil { return nil }

	items, ok := data["data"].(map[string]interface{})["items"].([]interface{})
	if !ok { return nil }

	var movies []map[string]interface{}
	for _, item := range items {
		i, ok := item.(map[string]interface{})
		if !ok { continue }
		cover := ""
		if c, ok := i["cover"].(map[string]interface{}); ok && c != nil {
			if u, ok := c["url"].(string); ok { cover = u }
		}
		movies = append(movies, map[string]interface{}{
			"title": i["title"], "poster": cover, "slug": i["detailPath"],
			"id": i["subjectId"], "year": i["releaseDate"], "source": "moviebox.ph",
			"hasStream": i["hasResource"],
		})
	}
	return map[string]interface{}{"source": "moviebox.ph", "count": len(movies), "movies": movies}
}

func search2Embed(q string) interface{} {
	resp, err := client.Get("https://api.2embed.cc/search?q=" + q)
	if err != nil { return nil }
	defer resp.Body.Close()

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	results, ok := data["results"].([]interface{})
	if !ok { return nil }

	var movies []map[string]interface{}
	for _, r := range results {
		m, ok := r.(map[string]interface{})
		if !ok { continue }
		movies = append(movies, map[string]interface{}{
			"title": m["title"], "year": m["year"], "imdbID": m["imdb_id"],
			"poster": m["poster_path"], "source": "2embed.cc",
			"embed": fmt.Sprintf("https://www.2embed.cc/embed/%s", m["imdb_id"]),
		})
	}
	return map[string]interface{}{"source": "2embed.cc", "count": len(movies), "movies": movies}
}

func handleWatch(w http.ResponseWriter, r *http.Request) {
	cors(w)
	id := r.URL.Query().Get("id")
	path := r.URL.Query().Get("path")
	quality := r.URL.Query().Get("quality")
	se := r.URL.Query().Get("se")
	ep := r.URL.Query().Get("ep")
	if id == "" || path == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "id and path required"})
		return
	}
	if se == "" { se = "0" }
	if ep == "" { ep = "0" }

	// Get stream URLs
	url := fmt.Sprintf("%s/wefeed-h5api-bff/subject/play?subjectId=%s&se=%s&ep=%s&detailPath=%s", NETFILM, id, se, ep, path)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", UA)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", fmt.Sprintf("%s/spa/videoPlayPage/movies/%s", NETFILM, path))
	req.Header.Set("Origin", NETFILM)
	req.Header.Set("Cookie", fmt.Sprintf("uuid=%s; token=%s", UUID, JWT_TOKEN))

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var data map[string]interface{}
	json.Unmarshal(body, &data)

	streams, ok := data["data"].(map[string]interface{})["streams"].([]interface{})
	if !ok || len(streams) == 0 {
		http.Error(w, "No streams available", 404)
		return
	}

	// Pick the right quality
	var streamUrl string
	for _, s := range streams {
		stream := s.(map[string]interface{})
		if quality == "" || fmt.Sprint(stream["resolutions"]) == quality {
			streamUrl = fmt.Sprint(stream["url"])
			break
		}
	}
	if streamUrl == "" {
		streamUrl = fmt.Sprint(streams[0].(map[string]interface{})["url"])
	}

	// Proxy the video
	proxyStream(w, r, streamUrl)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	cors(w)
	id := r.URL.Query().Get("id")
	path := r.URL.Query().Get("path")
	quality := r.URL.Query().Get("quality")
	se := r.URL.Query().Get("se")
	ep := r.URL.Query().Get("ep")
	if id == "" || path == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "id and path required"})
		return
	}
	if se == "" { se = "0" }
	if ep == "" { ep = "0" }

	url := fmt.Sprintf("%s/wefeed-h5api-bff/subject/play?subjectId=%s&se=%s&ep=%s&detailPath=%s", NETFILM, id, se, ep, path)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", UA)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", fmt.Sprintf("%s/spa/videoPlayPage/movies/%s", NETFILM, path))
	req.Header.Set("Origin", NETFILM)
	req.Header.Set("Cookie", fmt.Sprintf("uuid=%s; token=%s", UUID, JWT_TOKEN))

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var data map[string]interface{}
	json.Unmarshal(body, &data)

	streams, ok := data["data"].(map[string]interface{})["streams"].([]interface{})
	if !ok || len(streams) == 0 {
		http.Error(w, "No streams available", 404)
		return
	}

	var streamUrl string
	for _, s := range streams {
		stream := s.(map[string]interface{})
		if quality == "" || fmt.Sprint(stream["resolutions"]) == quality {
			streamUrl = fmt.Sprint(stream["url"])
			break
		}
	}
	if streamUrl == "" {
		streamUrl = fmt.Sprint(streams[0].(map[string]interface{})["url"])
	}

	// Force download with filename
	proxyDownload(w, r, streamUrl, fmt.Sprintf("movie-%s-%sp.mp4", id, quality))
}

func proxyStream(w http.ResponseWriter, r *http.Request, targetURL string) {
	req, _ := http.NewRequest("GET", targetURL, nil)
	req.Header.Set("User-Agent", UA)
	req.Header.Set("Referer", NETFILM+"/")
	req.Header.Set("Origin", NETFILM)

	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		w.Header().Set("Content-Range", cr)
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func proxyDownload(w http.ResponseWriter, r *http.Request, targetURL, filename string) {
	req, _ := http.NewRequest("GET", targetURL, nil)
	req.Header.Set("User-Agent", UA)
	req.Header.Set("Referer", NETFILM+"/")
	req.Header.Set("Origin", NETFILM)

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
