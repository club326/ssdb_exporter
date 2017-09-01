package exporter

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/gossdb/ssdb"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"strings"
	"sync"
	"time"
)

//SsdbHost represents a set of Ssdb Hosts to health check
type SsdbHost struct {
	Addrs     []string
	Passwords []string
}

type dbKeyPair struct {
	db, key string
}

//Exporter implements prometheus.Exporter interface and exports Ssdb metrics.
type Exporter struct {
	ssdb         SsdbHost
	namespace    string
	keys         []string
	keyValues    *prometheus.GaugeVec
	duration     prometheus.Gauge
	scrapeErrors prometheus.Gauge
	totalScrapes prometheus.Counter
	metrics      map[string]*prometheus.GaugeVec
	metricsMtx   sync.RWMutex
	sync.RWMutex
}

type scrapeResult struct {
	Name   string
	Value  float64
	Addr   string
	Result string
}

var (
	namespace = "ssdb"
	version   = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "version",
		Help:      "The version of ssdb",
	}, []string{"addr"})
	masterHost = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "master_host",
		Help:      "master_host of ssdb",
	}, []string{"addr"})
	replication_id = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "replication_id",
		Help:      "replication id of ssdb",
	}, []string{"addr"})
	replication_type = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "replication_type",
		Help:      "replication type of ssdb",
	}, []string{"addr"})
	replication_status = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "replication_status",
		Help:      "replication status of ssdb",
	}, []string{"addr"})
)

//NewSsdbExporter returns a new exporter of Ssdb metrics.
func NewSsdbExporter(host SsdbHost, namespace string) (*Exporter, error) {
	e := Exporter{
		ssdb:      host,
		namespace: namespace,
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "exporter_last_scrape_duration_seconds",
			Help:      "The last scrape duration.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrapes_total",
			Help:      "Current total redis scrapes.",
		}),
		scrapeErrors: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "exporter_last_scrape_error",
			Help:      "The last scrape error status.",
		}),
	}
	return &e, nil
}

//Describe outputs Ssdb metric descriptions.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range e.metrics {
		m.Describe(ch)
	}
	ch <- e.duration.Desc()
	ch <- e.totalScrapes.Desc()
	ch <- e.scrapeErrors.Desc()
}

//Collect fetches new metrics from Ssdbhost and updates the approriate metrics.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	scrapes := make(chan scrapeResult)
	e.Lock()
	defer e.Unlock()
	go e.scrape(scrapes)
	e.setMetrics(scrapes)
	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.scrapeErrors
	e.collectMetrics(ch)
}

//extract info into metrics from ssdb
func (e *Exporter) extractInfoMetrics(lines []string, addr string, scrapes chan<- scrapeResult) error {
	//lines := strings.Split(info, "\r\n")
	for key, value := range lines {
		fmt.Printf("info:%s", value)
		log.Debugf("info:%s", value)
		switch value {
		case "version":
			key += 1
			ver := lines[key]
			ls := prometheus.Labels{
				"version": ver,
				"addr":    addr,
			}
			version.With(ls).Set(1)
			// scrapes <- scrapeResult{Name: "version", Addr: addr, Value: val}
		case "links":
			key += 1
			valu := lines[key]
			if val, err := strconv.ParseFloat(valu, 64); err == nil {
				scrapes <- scrapeResult{Name: "links", Addr: addr, Value: val}
			} else {
				log.Printf("parse err:%s", err)
				return err
			}
		case "total_calls":
			key += 1
			valu := lines[key]
			if val, err := strconv.ParseFloat(valu, 64); err == nil {
				scrapes <- scrapeResult{Name: "total_calls", Addr: addr, Value: val}
			} else {
				log.Printf("parse err:%s", err)
				return err
			}
		case "binlogs":
			key += 1
			binlogs := strings.Split(lines[key], "\n")
			for _, v := range binlogs {
				v = strings.Replace(v, " ", "", -1)
				results := strings.Split(v, ":")
				if len(results) != 2 {
					continue
				}
				switch results[0] {
				case "capacity":
					valu := strings.Replace(results[1], " ", "", -1)
					if val, err := strconv.ParseFloat(valu, 64); err == nil {
						scrapes <- scrapeResult{Name: "binlogs_capacity", Addr: addr, Value: val}
					} else {
						log.Printf("parse err:%s", err)
						return err
					}
				case "min_seq":
					valu := strings.Replace(results[1], " ", "", -1)
					if val, err := strconv.ParseFloat(valu, 64); err == nil {
						scrapes <- scrapeResult{Name: "binlogs_min_sql", Addr: addr, Value: val}
					} else {
						log.Printf("parse err:%s", err)
						return err
					}
					//scrapes <- scrapeResult{Name: "binlogs_min_seq", Addr: addr, Value: strconv.ParseInt(val, 10, 64)}
				case "max_seq":
					valu := strings.Replace(results[1], " ", "", -1)
					if val, err := strconv.ParseFloat(valu, 64); err == nil {
						scrapes <- scrapeResult{Name: "binlogs_max_seq", Addr: addr, Value: val}
					} else {
						log.Printf("parse err:%s", err)
						return err
					}
					//scrapes <- scrapeResult{Name: "binlogs_max_seq", Addr: addr, Value: strconv.ParseInt(val, 10, 64)}
				}
			}
		case "replication":
			key += 1
			if strings.Contains(lines[key], "slaveof") {
				replication := strings.Split(lines[key], "\n")
				for _, repV := range replication {
					if strings.Contains(repV, "slaveof") {
						slaveof := strings.Replace(strings.Replace(repV, "slaveof", "", -1), " ", "", -1)
						labels := prometheus.Labels{
							"masterHost": slaveof,
							"addr":       addr,
						}
						masterHost.With(labels).Set(1)
						//scrapes <- scrapeResult{Name: "slaveof", Addr: addr, Value: strconv.ParseInt(val, 10, 64)}
					} else {
						repArray := strings.Split(strings.Replace(repV, " ", "", -1), ":")
						if len(repArray) != 2 {
							continue
						}
						switch repArray[0] {
						case "id":
							replicationId := strings.Replace(string(repV[1]), " ", "", -1)
							labels := prometheus.Labels{
								"replication_id": replicationId,
								"addr":           addr,
							}
							replication_id.With(labels).Set(1)
							//scrapes <- scrapeResult{Name: "replication_id", Addr: addr, Value: strconv.ParseInt(val, 10, 64)}
						case "type":
							replicationType := strings.Replace(string(repV[1]), " ", "", -1)
							labels := prometheus.Labels{
								"replication_type": replicationType,
								"addr":             addr,
							}
							replication_type.With(labels).Set(1)
							//scrapes <- scrapeResult{Name: "replication_type", Addr: addr, Value: strconv.ParseInt(val, 10, 64)}
						case "status":
							replicationStatus := strings.Replace(string(repV[1]), " ", "", -1)
							//scrapes <- scrapeResult{Name: "replication_status", Addr: addr, Value: strconv.ParseInt(val, 10, 64)}
							labels := prometheus.Labels{
								"replication_status": replicationStatus,
								"addr":               addr,
							}
							replication_status.With(labels).Set(1)
						case "last_seq":
							valu := strings.Replace(string(repV[1]), " ", "", -1)
							if val, err := strconv.ParseFloat(valu, 64); err == nil {
								scrapes <- scrapeResult{Name: "replication_last_seq", Addr: addr, Value: val}
							} else {
								log.Printf("parse err:%s", err)
								return err
							}
						case "copy_count":
							valu := strings.Replace(string(repV[1]), " ", "", -1)
							if val, err := strconv.ParseFloat(valu, 64); err == nil {
								scrapes <- scrapeResult{Name: "replication_copy_count", Addr: addr, Value: val}
							} else {
								log.Printf("parse err:%s", err)
								return err
							}
							//scrapes <- scrapeResult{Name: "replication_copy_count", Addr: addr, Value: strconv.ParseInt(val, 10, 64)}
						case "sync_count":
							valu := strings.Replace(string(repV[1]), " ", "", -1)
							if val, err := strconv.ParseFloat(valu, 64); err == nil {
								scrapes <- scrapeResult{Name: "replication_sync_count", Addr: addr, Value: val}
							} else {
								log.Printf("parse err:%s", err)
								return err
							}
							//scrapes <- scrapeResult{Name: "replication_sync_count", Addr: addr, Value: strconv.ParseInt(val, 10, 64)}

						}
					}
				}
			}
		}

	}
	return nil
}

//trying to execute info command to get metrics from ssdb
func (e *Exporter) scrapeSsdbHost(scrapes chan<- scrapeResult, addr string, idx int) error {

	log.Debugf("Trying to connect ssdb:%s", addr)
	host := strings.Split(addr, ":")[0]
	portT, err := strconv.ParseInt(strings.Split(addr, ":")[1], 10, 32)
	port := int(portT)
	if err != nil {
		log.Printf("trying to parse port to int err:%s", err)
		return err
	}
	s, err := ssdb.Connect(host, port)
	defer ssdb.Close()

	if err != nil {
		log.Printf("trying to connect ssdb err: %s", err)
		return err
	}
	log.Debugf("connected to %s", addr)

	info, err := s.Do("INFO")
	if err == nil {
		e.extractInfoMetrics(info, addr, scrapes)
		//log.Printf("ssdb info:%s", info)
	} else {
		log.Printf("ssdb error:%s", err)
		return err
	}
	return nil
}

func (e *Exporter) scrape(scrapes chan<- scrapeResult) {
	defer close(scrapes)
	now := time.Now().UnixNano()
	e.totalScrapes.Inc()
	errCount := 0
	for idx, addr := range e.ssdb.Addrs {
		var up float64 = 1
		if err := e.scrapeSsdbHost(scrapes, addr, idx); err != nil {
			errCount++
			up = 0
		}
		scrapes <- scrapeResult{Name: "up", Addr: addr, Value: up}
	}
	e.scrapeErrors.Set(float64(errCount))
	e.duration.Set(float64(time.Now().UnixNano()-now) / 10000)
}

func (e *Exporter) setMetrics(scrapes <-chan scrapeResult) {
	for src := range scrapes {
		name := src.Name
		if _, ok := e.metrics[name]; !ok {
			e.metricsMtx.Lock()
			e.metrics[name] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: e.namespace,
				Name:      name,
			}, []string{"addr"})
			e.metricsMtx.Unlock()
		}
		var labels prometheus.Labels = map[string]string{"addr": src.Addr}
		e.metrics[name].With(labels).Set(src.Value)
	}
}

func (e *Exporter) collectMetrics(metrics chan<- prometheus.Metric) {
	for _, m := range e.metrics {
		m.Collect(metrics)
	}
}
