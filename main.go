package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type SongLink struct {
	Href  string
	Title string
}

func main() {
	start := time.Now()
	defer func() {
		log.Println(time.Since(start))
	}()

	_, err := exec.LookPath("yt-dlp")
	if err != nil {
		log.Fatal("yt-dlp missing")
	}

	links, err := getNjoySongsChart()
	if err != nil {
		log.Fatal(err)
	}
	_, err = os.ReadDir("mp3s")

	if errors.Is(err, os.ErrNotExist) {
		log.Println("creaing mp3s folder")
		os.Mkdir("mp3s", os.ModeDir)
	}

	for i, link := range links {
		if err := downloadSong(link, i, links); err != nil {
			fmt.Println(err)
		}
	}
}

func downloadSong(link SongLink, i int, links []SongLink) error {
	if !strings.HasPrefix(link.Href, "https://www.youtube.com/watch") {
		log.Printf("[%02d/%02d] skipping, not a valid youtube link '%s'", i+1, len(links), link.Href)
		return nil
	}

	fp := path.Join("mp3s", link.Title+".mp3")
	if _, err := os.Stat(fp); !errors.Is(err, os.ErrNotExist) {
		log.Printf("[%02d/%02d] already downloaded '%s'\n", i+1, len(links), link.Title)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()
	log.Printf("[%02d/%02d] downloading %s\n", i+1, len(links), link.Title)
	cmd := exec.CommandContext(ctx, "yt-dlp", "-f", "ba", "-o", fp, link.Href)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("[%02d/%d] failed to download '%s', %s", i+1, len(links), link.Title, stderr.String())
	}
	return nil
}

func getNjoySongsChart() ([]SongLink, error) {
	url := "https://njoy.bg/charts/2.html"
	links := make([]SongLink, 0, 40)
	rpl := strings.NewReplacer("'", "", ",", "")
	resp, err := http.Get(url)
	if err != nil {
		return links, errors.New("failed to open the njoy chart url")
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return links, errors.New("failed to read the response body")
	}

	rows := doc.Find(".rnk2-row")
	for _, row := range rows.EachIter() {
		var link SongLink
		link.Href = row.Find(".rnk-yt").AttrOr("href", "")
		link.Title = rpl.Replace(row.Find(".rnk2-artist").Text() + " - " + row.Find(".rnk2-song").Text())
		links = append(links, link)
	}

	return links, nil
}
