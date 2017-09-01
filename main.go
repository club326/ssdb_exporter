package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	"github.com/club326/ssdb_exporter/exporter"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"os"
	"runtime"
	"strings"
)

var (
	ssdbAddr      = flag.String("ssdb.addr", getEnv("SSDB_ADDR", "ssdb://localhost:8001"), "Address of one or more redis nodes,seprated by separator")
	ssdbPassword  = flag.String("ssdb.password", getEnv("SSDB_PASSWORD", ""), "Password for one or more ssdb node")
	namespace     = flag.String("namespace", "ssdb", "Namespace for metrics")
	separator     = flag.String("separator", ",", "separator used to split ssdb.addr, ssdb.password into several elements.")
	listenAddress = flag.String("web.listen-address", ":9141", "Address to listen on for web interface and telemetry.")
	metricPath    = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	isDebug       = flag.Bool("debug", false, "Output verbose debug information")
	logFormat     = flag.String("log-format", "txt", "Log format, valid options are txt and json")
	showVersion   = flag.Bool("version", false, "Show version information and exit")
	VERSION       = "1.0test"
	BUILD_DATE    = "<<< filled in by build >>>"
)

func main() {
	flag.Parse()
	switch *logFormat {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	default:
		log.SetFormatter(&log.TextFormatter{})
	}
	log.Printf("ssdb Metrics Exporter %s build date:%s  Go:%s\n",
		VERSION, BUILD_DATE, runtime.Version(),
	)
	if *isDebug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	if *showVersion {
		return
	}
	addrs := strings.Split(*ssdbAddr, *separator)
	passwords := strings.Split(*ssdbPassword, *separator)
	if len(passwords) < len(addrs) {
		passwords = append(passwords, passwords[0])
	}
	exp, err := exporter.NewSsdbExporter(
		exporter.SsdbHost{Addrs: addrs, Passwords: passwords},
		*namespace)
	if err != nil {
		log.Fatal(err)
	}
	prometheus.MustRegister(exp)

	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ssdb_exporter_build_info",
		Help: "ssdb exporter build info",
	}, []string{"version", "build_date", "golang_version"})
	prometheus.MustRegister(buildInfo)
	buildInfo.WithLabelValues(VERSION, BUILD_DATE, runtime.Version()).Set(1)
	http.Handle(*metricPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
<html>
<head><title>Redis Exporter v` + VERSION + `</title></head>
<body>
<h1><a href='` + *metricPath + `'>Metrics</a></p>
</body>
</html>
`))
	})
	log.Printf("Providing metrics at %s%s", *listenAddress, *metricPath)
	log.Printf("Connecting to ssdb host:%#v", addrs)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
func getEnv(key string, defaultVal string) string {
	if envVal, ok := os.LookupEnv(key); ok {
		return envVal
	}
	return defaultVal
}
