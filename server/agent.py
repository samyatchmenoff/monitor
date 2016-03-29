#!/usr/bin/env python3

import time
import datetime
import json
import requests
import psutil

while True:
  cpu_times = psutil.cpu_times_percent()
  net_io_counters = psutil.net_io_counters(pernic=True)
  metrics = {}
  metrics['cpu.user'] = cpu_times.user
  metrics['cpu.nice'] = cpu_times.nice
  metrics['cpu.system'] = cpu_times.system
  metrics['cpu.idle'] = cpu_times.idle
  for k in net_io_counters:
    metrics['net.if.{}.bytes_sent'.format(k)] = net_io_counters[k].bytes_sent
    metrics['net.if.{}.bytes_recv'.format(k)] = net_io_counters[k].bytes_recv
    metrics['net.if.{}.packets_sent'.format(k)] = net_io_counters[k].packets_sent
    metrics['net.if.{}.packets_recv'.format(k)] = net_io_counters[k].packets_recv
    metrics['net.if.{}.errin'.format(k)] = net_io_counters[k].errin
    metrics['net.if.{}.errout'.format(k)] = net_io_counters[k].errout
    metrics['net.if.{}.dropin'.format(k)] = net_io_counters[k].dropin
    metrics['net.if.{}.dropout'.format(k)] = net_io_counters[k].dropout
  req = {
    'resource_id': 'test',
    'timestamp': datetime.datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ'),
    'metrics': metrics
  }
  requests.post('http://localhost:5000/metrics', data=json.dumps(req))
  time.sleep(1)
