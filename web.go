package cpuprofile

import (
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"strconv"
	"time"
)

// ProfileHTTPHandler is same as pprof.Profile.
// The difference is ProfileHTTPHandler uses cpuprofile.GetCPUProfile to fetch profile data.
func profileHTTPHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	sec, err := strconv.ParseInt(r.FormValue("seconds"), 10, 64)
	if sec <= 0 || err != nil {
		sec = 5
	}
	if durationExceedsWriteTimeout(r, float64(sec)) {
		serveError(w, http.StatusBadRequest, "profile duration exceeds server's WriteTimeout")
		return
	}

	// Set Content Type assuming StartCPUProfile will work,
	// because if it does it starts writing.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="profile"`)

	pc := NewCollector()
	err = pc.StartCPUProfile(w)
	if err != nil {
		serveError(w, http.StatusInternalServerError, "Could not enable CPU profiling: "+err.Error())
		return
	}
	time.Sleep(time.Second * time.Duration(sec))
	err = pc.StopCPUProfile()
	if err != nil {
		serveError(w, http.StatusInternalServerError, "Could not enable CPU profiling: "+err.Error())
	}
}

func durationExceedsWriteTimeout(r *http.Request, seconds float64) bool {
	srv, ok := r.Context().Value(http.ServerContextKey).(*http.Server)
	return ok && srv.WriteTimeout != 0 && seconds >= srv.WriteTimeout.Seconds()
}

func serveError(w http.ResponseWriter, status int, txt string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Go-Pprof", "1")
	w.Header().Del("Content-Disposition")
	w.WriteHeader(status)
	_, err := fmt.Fprintln(w, txt)
	if err != nil {
		log.Printf("write http response error %v", err)
	}
}

func WebProfile(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", profileHTTPHandler)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	// Other /debug/pprof paths not covered above are redirected to pprof.Index.
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	go func() {
		log.Println(http.ListenAndServe(addr, mux))
	}()
}