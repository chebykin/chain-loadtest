#!/usr/bin/python

import sys
import cpuinfo

# https://github.com/workhorsy/py-cpuinfo
cpu = cpuinfo.get_cpu_info()

print sys.argv[1] + ':'
print '    name: ' + format(cpu['brand'])
print '    freq: ' + format(cpu['hz_actual_raw'][0]/1000000)
print '    family: ' + format(cpu['family'])
print '    model: ' + format(cpu['model'])
print '    cores: ' + format(cpu['count'])