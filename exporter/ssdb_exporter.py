from prometheus_client import start_http_server,Gauge
import pyssdb
import time


ssdb_up = Gauge("ssdb_up","whether ssdb up or not ",['addr'])
ssdb_version = Gauge("ssdb_version","the version of ssdb",['addr','version'])
ssdb_links = Gauge("ssdb_links","the links of ssdb server",['addr'])
ssdb_total_calls = Gauge("ssdb_total_calls","total_calls of ssdb server",['addr'])
ssdb_dbsize = Gauge("ssdb_dbsize","dbsize of ssdb server",['addr'])
ssdb_capacity = Gauge("ssdb_capacity","the binlogs capacity of ssdb server",['addr'])
ssdb_min_seq = Gauge("ssdb_min_seq","the binlogs min_seq of ssdb server",['addr'])
ssdb_max_seq = Gauge("ssdb_max_seq","the binlogs max_seq of ssdb server",['addr'])
replication_master_host = Gauge("ssdb_replication_master_host","the replication master of ssdb server",['addr','replication_master_host'])
replication_slave_host = Gauge("ssdb_replication_slave_host","the replication master of ssdb server",['addr','replication_slave_host'])
replication_id = Gauge("ssdb_replication_id","the replication id of ssdb server",['addr','replication_id'])
replication_type = Gauge("ssdb_replication_type","the replication type of ssdb server",['addr','replication_type'])
replication_status = Gauge("ssdb_replication_status","the replication status of ssdb server",['addr','replication_status'])
replication_last_seq = Gauge("ssdb_replication_last_seq","the replication last seq of ssdb server",['addr'])
replication_copy_count = Gauge("ssdb_replication_copy_count","the replication copy count of ssdb server",['addr'])
replication_sync_count = Gauge("ssdb_replication_sync_count","the replication sync count of ssdb server",['addr'])
g = Gauge('my_inprogress_requests', 'Description of gauge')
g.set_to_current_time()
@g.track_inprogress()
def check_ssdb_config(host,port):
    try:
        addr = "%s:%s" %(host,port)
        ssdb = pyssdb.Client(host = host,port = port)
        info = ssdb.info()
        up = 1
        #ssdb_up = Gauge("ssdb_up","whether ssdb is up or not",['addr'])
        ssdb_up.labels(addr).set(up)
        for i in range(0,len(info)):
            if info[i] == "version":
                i += 1
                version = info[i]
                ssdb_version.labels(addr,version).set(1)
            elif info[i] == "links":
                i += 1
                links = info[i]
                ssdb_links.labels(addr).set(links)
            elif info[i] == "total_calls":
                i += 1
                total_calls = info[i]
                ssdb_total_calls.labels(addr).set(total_calls)
            elif info[i] == "dbsize":
                i += 1
                dbsize = info[i]
                ssdb_dbsize.labels(addr).set(dbsize)
            elif info[i] == "binlogs":
                i += 1
                binlogs = info[i]
                binlogs = binlogs.split('\n')
                for line in binlogs:
                    line = line.strip().split(':')
                    if line[0].find("capacity") != -1:
                        capacity = line[1].strip()
                        ssdb_capacity.labels(addr).set(capacity)
                    elif line[0].find("min_seq") != -1:
                        min_seq = line[1].strip()
                        ssdb_min_seq.labels(addr).set(min_seq)
                    elif line[0].find("max_seq") != -1:
                        max_seq = line[1].strip()   
                        ssdb_max_seq.labels(addr).set(max_seq)
            elif info[i] == "replication":
                i += 1
                if info[i].find("slaveof") != -1:
                    master_host,id,type,status,last_seq,copy_count,sync_count = '','','','','','',''
                    replication = info[i].split("\n")
                    for line in replication:
                        if line.find("slaveof") != -1:
                             master_host = line.replace('slaveof','').strip()
                             replication_master_host.labels(addr,master_host).set(1)
                        else:
                             line = line.strip().split(':')
                             if line[0].find("id") != -1:
                                 id = line[1].strip()
                                 replication_id.labels(addr,id).set(1)
                             elif line[0].find("type") != -1:
                                 type = line[1].strip()
                                 replication_type.labels(addr,type).set(1)
                             elif line[0].find("status") != -1:
                                 status = line[1].strip()
                                 replication_status.labels(addr,status).set(1)
                             elif line[0].find("last_seq") != -1:
                                 last_seq = line[1].strip()
                                 replication_last_seq.labels(addr).set(last_seq)
                             elif line[0].find("copy_count") != -1:
                                 copy_count = line[1].strip()
                                 replication_copy_count.labels(addr).set(copy_count)
                             elif line[0].find("sync_count") != -1:
                                 sync_count = line[1].strip()
                                 replication_sync_count.labels(addr).set(sync_count)
                elif info[i].find("client") != -1:
                    client_status = info[i].split("\n")
                    client_host,status,last_seq = '','',''
                    for line in client_status:
                        if line.find("client") != -1:
                            client_host = line.replace("client",'')
                            replication_slave_host.labels(addr,client_host).set(1)
                        else:
                            line = line.strip().split(':')
                            if line[0].find("status") != -1:
                                status = line[1].strip()
                                replication_status.labels(addr,status).set(1)
                            elif line[0].find("last_seq") != -1:
                                 last_seq = line[1].strip()
                                 replication_last_seq.labels(addr).set(last_seq)

        ssdb.disconnect()
    except Exception,e:
        up=0
        ssdb_up.labels(addr).set(up)

if __name__ == '__main__':
    start_http_server(9303,addr='192.168.11.103')
    #check_ssdb_config('192.168.11.103',8888)
    #check_ssdb_config('192.168.11.103',8001)
    #check_ssdb_config('192.168.11.103',8002)
    while True:
        check_ssdb_config('192.168.11.103',8888)
        #check_ssdb_config('10.4.0.20',8001)
        #check_ssdb_config('10.4.0.20',8002)
        time.sleep(30)