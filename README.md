# ring
Ring is a simple ring buffer that allows `put()` and `get()` of an int. A `put()` can return "full" and a `get()`can return "empty". The implementation uses a 64 bit atomic comapre and swap instruction which should make it go routine safe but the code has only been tested with a single producer and consumer.

## performance
The example code currently benchmarks the ring buffer at just under 8 Gops/sec vs about 13 Gops/sec for channels so there seems to be no advantage to using a ring buffer versus channels.

	leb@hula-3:~/gotest/src/leb.io/ring/example % time ./example -s=12
	6.51 Gops/sec
	
	leb@hula-3:~/gotest/src/leb.io/ring/example % time ./example -s=12 -cf
	13.12 Gops/sec
	
Getting rid of the atomic statistic counters and ALL statistic counters more than doubled perforance from 3 Gops/sec to almost 8 Gops/sec.
