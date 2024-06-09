package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/kkdai/youtube/v2"
)

func main() {
	start := time.Now()
	defer func() {
		fmt.Println(time.Since(start))
	}()

	links, err := getNjoySongsChart()
	if err != nil {
		log.Fatal(err)
	}

	_, err = os.ReadDir("mp3s")

	if errors.Is(err, os.ErrNotExist) {
		os.Mkdir("mp3s", os.ModeDir)
	}

	vIDc := make(chan string, 40)
	rx := regexp.MustCompile(`\([A-Za-z0-9\s]+\)|\[[A-Za-z0-9\s]+\]|[?\/|]`)
	var wg sync.WaitGroup

	wg.Add(len(links))

	for i := 0; i < 4; i++ {
		go downloadSong(vIDc, rx, &wg)
	}

	for _, link := range links {
		vIDc <- link
	}

	wg.Wait()
}

func getNjoySongsChart() ([]string, error) {
	url := "https://njoy.bg/charts/listing/?chart_id=3"
	var links []string

	resp, err := http.Get(url)
	if err != nil {
		return links, errors.New("failed to open the njoy chart url")
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return links, errors.New("failed to read the response body")
	}

	s := doc.Find(".songs-container")
	rawLinks := s.Find(".buttons > .article_youtube_link > span")

	links = rawLinks.Map(func(i int, s *goquery.Selection) string {
		return s.Text()
	})

	return links, nil
}

func downloadSong(vIDc chan string, r *regexp.Regexp, wg *sync.WaitGroup) {
	client := youtube.Client{}

	for vID := range vIDc {
		func(vID string) {
			video, err := client.GetVideo(vID)
			if err != nil {
				if vID == "" {
					log.Println("skipping empty link download")
					wg.Done()
					return
				}
				log.Println(err, "retrying...", vID)
				vIDc <- vID
				return
			}

			fName := strings.TrimSpace(r.ReplaceAllString(video.Title, ""))
			filePath := fmt.Sprintf("mp3s/%s.mp3", fName)

			if doesPathExists(filePath) {
				log.Printf("%s is already donwloaded.\n", fName)
				wg.Done()
				return
			}

			formats := video.Formats.WithAudioChannels().Type("audio/webm")
			stream, _, err := client.GetStream(video, &formats[0])
			if err != nil {
				log.Println(err, "retrying...")
				vIDc <- vID
				return

			}
			defer stream.Close()

			file, err := os.Create(fmt.Sprintf("mp3s/%s.mp3", fName))
			if err != nil {
				log.Println(err, "retrying...")
				vIDc <- vID
				return

			}
			defer file.Close()

			_, err = io.Copy(file, stream)
			if err != nil {
				log.Println(err, "retrying...")
				vIDc <- vID
				return

			}

			log.Printf("downloaded: %s to the mp3s folder\n", fName)
			wg.Done()
		}(vID)
	}
}

func doesPathExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return false
	}

	return true
}
