package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nareix/joy5/format/flv"
	"github.com/nareix/joy5/format/rtmp"
)

const (
	DefaultUserAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var appKey string

func getRealRtmp(url string) string {
	client := &http.Client{
		Timeout: time.Second * 10,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	redir := resp.Header.Get("Location")
	if redir == "" {
		return ""
	}
	return redir
}

func serveFlv(w http.ResponseWriter, r *http.Request) {
	rtmpUrl := r.URL.Query().Get("rtmp")
	userKey := r.URL.Query().Get("appkey")
	if appKey != "" && userKey != appKey {
		w.WriteHeader(404)
		return
	}
	u, err := url.Parse(rtmpUrl)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	rtmpAddr := rtmpUrl
	scheme := strings.ToLower(u.Scheme)
	if scheme != "rtmp" {
		rtmpAddr = getRealRtmp(rtmpUrl)
	}
	if rtmpAddr == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid url"))
		return
	}
	log.Println("Forwarding", rtmpAddr)
	rtmpConn, conn, err := rtmp.NewClient().Dial(rtmpAddr, rtmp.PrepareReading)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(err.Error()))
		return
	}

	defer conn.Close()
	w.Header().Set("Content-Type", "video/x-flv")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(200)
	flusher := w.(http.Flusher)
	flusher.Flush()

	muxer := flv.NewMuxer(w)
	muxer.WriteFileHeader()
	for {
		packet, err := rtmpConn.ReadPacket()
		if err != nil {
			log.Println("stream ended with error", err)
			break
		}
		muxer.WritePacket(packet)
	}
	return
}

func main() {
	listen := flag.String("flv", ":80", "flv dest address")
	flag.StringVar(&appKey, "appkey", "", "optional appkey for more privacy")
	flag.Parse()

	if *listen == "" {
		log.Fatalln("Flv address can not be empty")
	}

	log.Println("rtmp to flv server started")

	mux := http.NewServeMux()
	mux.HandleFunc("/flv", serveFlv)
	err := http.ListenAndServe(*listen, mux)
	if err != nil {
		log.Fatalln(err)
	}

	// flvWriter := flv.NewWriter(flvFile)
	// for {
	//     packet, err := rtmpConn.ReadPacket()
	//     if err != nil {
	//         break
	//     }
	//     flvWriter.WritePacket(packet)
	// }
	// flvWriter.Close()
}
