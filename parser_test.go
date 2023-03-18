package main

import (
	"reflect"
	"testing"
)

const SeedConfig = `
	lease 192.168.10.20 {
		starts 5 2020/03/27 15:27:49 UTC;
		ends 6 2020/03/28 03:27:49 UTC;
		hardware ethernet 00:01:02:03:04:05;
		client-hostname "fuzzy";
	}

 	 lease 192.168.10.20 {
		starts 5 2020/03/27 15:27:49 UTC;
		ends 6 2020/03/28 03:27:49 UTC;
		hardware ethernet 00:01:02:03:04:05;
		client-hostname "fuzzy";
	}
`

func FuzzLeaseParser(t *testing.F) {
	t.Add(1024, []byte(SeedConfig))
	t.Add(1, []byte(SeedConfig))
	t.Fuzz(func(t *testing.T, blockSize int, data []byte) {
		if blockSize < 1 {
			return
		}

		lp1 := &leaseParser{}
		lp1.AddData(data)

		var leases1 []Lease
		for {
			l, ok := lp1.ParseLease()
			if !ok {
				break
			}
			leases1 = append(leases1, l)
		}

		// feed lp2 with blocksize chunks
		lp2 := &leaseParser{}
		data2 := data[:]
		blockSize2 := blockSize
		var leases2 []Lease
		for {
			if blockSize2 >= len(data2) {
				blockSize2 = len(data2)
			}
			var block []byte
			block, data2 = data2[0:blockSize2], data2[blockSize2:]

			if len(block) == 0 {
				break
			}

			lp2.AddData(block)

			for {
				l, ok := lp2.ParseLease()
				if !ok {
					break
				}
				leases2 = append(leases2, l)
			}
		}

		equal := reflect.DeepEqual(leases1, leases2)
		if !equal {
			t.Errorf("parsed data not equal: blocksize %d\nleases1 %v\nleases2 %v\ndata %s\nrest %s", blockSize, leases1, leases2, string(data), string(data2))
		}
	})
}
