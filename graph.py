#!/usr/bin/python3
import matplotlib.pyplot as pp
from subprocess import Popen, PIPE
import sys
from settings import *

def main():
	if len(sys.argv) < 2:
		print("Нужен аргумент -- число итераций")
		exit(2)
	iters = int(sys.argv[1])

	graphData = []

	for prob in probs:
		print(prob)
		a = 0.0
		b = 0.0
		r = 0.0
		Popen("iptables -A INPUT -p udp -i lo -m statistic --mode random --probability {prob} -j DROP".format(prob=prob).split(), stdout=PIPE).communicate()
		for i in range(iters):
			print('\t'+str(i))
			res = Popen("./main {nodes} {port} {minDeg} {maxDeg} {ttl}".format(nodes=nodes, port=port, minDeg=minDeg, maxDeg=maxDeg, ttl=ttl).split(), stdout=PIPE).communicate()[0].split()
			if len(res) == 2 and res[1] == b"iters":
				a += float(res[0])
				b += 1
		try:
			r = a/b
		except ZeroDivisionError:
			r = 0.0
		graphData.append(r)
		Popen("iptables -D INPUT -p udp -i lo -m statistic --mode random --probability {prob} -j DROP".format(prob=prob).split(), stdout=PIPE).communicate()
	pp.plot(probs,graphData,'r')
	pp.xlabel("Probabilities of packet loss")
	pp.ylabel("Needed iter to finish")
	pp.title("TTL = {ttl}".format(ttl=ttl))
	pp.savefig("TTL = {ttl}".format(ttl=ttl))

if __name__ == "__main__":
	main()
