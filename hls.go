package hls

import (
	"errors"
	"io"
	"log"

	"net/http"
	"net/url"
	"os"

	"github.com/golang/groupcache/lru"
	"github.com/grafov/m3u8"
)

type Downloader interface {
	Download(playlistURL string, target string)
}

type HLSDownloader struct {
}

type stream struct {
	out   io.WriteCloser
	stURL string
	cache lru.Cache
}

func (s stream) loopDownloadStream() error {
	out, err := s.getPlaylist()
	if err != nil {
		return err
	}
	err = s.downloadSegments(out)
	return err
}

func (s stream) downloadSegments(list []string) error {
	for _, clip := range list {
		content, err := http.Get(clip)
		if err != nil {
			log.Print("Failed download " + err.Error())
			return err
		}
		defer content.Body.Close()

		_, err = io.Copy(s.out, content.Body)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s stream) getPlaylist() ([]string, error) {
	resp, err := http.Get(s.stURL)
	if err != nil {
		return nil, err
	}
	playlist, listType, _ := m3u8.DecodeFrom(resp.Body, true)

	if listType != m3u8.MEDIA {
		log.Println("Invalid media")
		return nil, errors.New("Invalid media type")
	}

	pl := playlist.(*m3u8.MediaPlaylist)
	out := s.addSegments(pl)

	return out, nil
}

func (s stream) addSegments(pl *m3u8.MediaPlaylist) []string {
	var out []string
	for _, seg := range pl.Segments {
		if seg != nil {
			segURL := s.toFullURL(seg.URI)
			_, inCache := s.cache.Get(segURL)
			if !inCache {
				s.cache.Add(segURL, nil)
				out = append(out, segURL)
			}
		}
	}
	return out
}

func (s stream) toFullURL(input string) string {
	sURL := s.getURL()
	newUrl, _ := sURL.Parse(input)
	return newUrl.String()
}

func newStream(playlistURL string, target string) (stream, error) {
	out, err := os.Create(target)
	if err != nil {
		return stream{}, err
	}

	st := stream{out, playlistURL, *lru.New(256)}
	return st, nil
}

func (s stream) getURL() *url.URL {
	out, _ := url.Parse(s.stURL)
	return out
}

func (HLSDownloader) Download(playlistURL string, target string) {
	s, err := newStream(playlistURL, target)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer s.out.Close()

	for {
		err := s.loopDownloadStream()
		if err != nil {
			return
		}
	}
}
