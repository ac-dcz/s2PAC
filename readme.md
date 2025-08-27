## s2PAC

[![golang](https://img.shields.io/badge/golang-1.21.1-blue?style=flat-square&logo=golang)](https://www.rust-lang.org)
[![python](https://img.shields.io/badge/python-3.9-blue?style=flat-square&logo=python&logoColor=white)](https://www.python.org/downloads/release/python-390/)
[![license](https://img.shields.io/badge/license-Apache-blue.svg?style=flat-square)](LICENSE)

This repo provides a minimal implementation of the s2PAC consensus protocol. The codebase has been designed to be small, efficient, and easy to benchmark and modify. It has not been designed to run in production but uses real cryptography (kyber), networking(native), and storage (nutsdb).

Say something about s2PAC...

## Quick Start

Wukong is written in Golang, but all benchmarking scripts are written in Python and run with Fabric. To deploy and benchmark a testbed of 4 nodes on your local machine, clone the repo and install the python dependencies:

```shell
git clone https://github.com/ac-dcz/s2pac
cd s2pac/benchmark
pip install -r requirements.txt
```

You also need to install tmux (which runs all nodes and clients in the background).
Finally, run a local benchmark using fabric:

```shell
fab local
```

This command may take a long time the first time you run it (compiling golang code in release mode may be slow) and you can customize a number of benchmark parameters in fabfile.py. When the benchmark terminates, it displays a summary of the execution similarly to the one below.

- s2PAC_Lean
```
-----------------------------------------
 SUMMARY:
-----------------------------------------
 + CONFIG:
 Protocol: 2pac_lean 
 DDOS attack: False 
 Committee size: 4 nodes
 Input rate: 3,200 tx/s
 Transaction size: 250 B
 Batch size: 200 tx/Batch
 Faults: 0 nodes
 Execution time: 30 s

 + RESULTS:
 Consensus TPS: 3,062 tx/s
 Consensus latency: 167 ms

 End-to-end TPS: 3,057 tx/s
 End-to-end latency: 221 ms

```

- s2PAC_Big
```
-----------------------------------------
 SUMMARY:
-----------------------------------------
 + CONFIG:
 Protocol: 2pac_big 
 DDOS attack: False 
 Committee size: 4 nodes
 Input rate: 3,200 tx/s
 Transaction size: 250 B
 Batch size: 200 tx/Batch
 Faults: 0 nodes
 Execution time: 30 s

 + RESULTS:
 Consensus TPS: 3,179 tx/s
 Consensus latency: 155 ms

 End-to-end TPS: 3,175 tx/s
 End-to-end latency: 207 ms
-----------------------------------------
```

- s2PAC-Big-DAG
```
-----------------------------------------
 SUMMARY:
-----------------------------------------
 + CONFIG:
 Protocol: 2pac_big_dag 
 DDOS attack: False 
 Committee size: 4 nodes
 Input rate: 3,200 tx/s
 Transaction size: 250 B
 Batch size: 200 tx/Batch
 Faults: 0 nodes
 Execution time: 30 s

 + RESULTS:
 Consensus TPS: 11,458 tx/s
 Consensus latency: 233 ms

 End-to-end TPS: 11,444 tx/s
 End-to-end latency: 283 ms
-----------------------------------------
```