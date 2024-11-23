import subprocess
from benchmark.commands import CommandMaker
from benchmark.local import LocalBench
from benchmark.logs import ParseError, LogParser
from benchmark.utils import BenchError,Print
from alibaba.instance import InstanceManager
from alibaba.remote import Bench
import sys

tasks = dict()

def task(f):
    tasks[f.__name__] = f
    return f

@task
def local():
    ''' Run benchmarks on localhost '''
    bench_params = {
        'nodes': [40],
        'duration': 25,
        'rate': [20_000],                  # tx send rate
        'batch_size': 4096,              # the max number of tx that can be hold 
        'log_level': 0b0001,            # 0x1 infolevel 0x2 debuglevel 0x4 warnlevel 0x8 errorlevel
        'protocol_name': "2pac_big_dag"
    }
    # [4_000,7_000,10_000,13_000,16_000,18_000,20_000,25_000]
    node_params = {
        "pool": {
            # "rate": 1_000,              # ignore: tx send rate 
            "tx_size": 32,               # tx size
            # "batch_size": 200,          # ignore: the max number of tx that can be hold 
            "max_queue_size": 100000 
	    },
        "consensus": {
            "sync_timeout": 500,        # node sync time
            "network_delay": 50,        # network delay
            "min_block_delay": 0,       # send block delay
            "ddos": False,              # DDOS attack
            "faults": 0,                # the number of byzantine node
            "protocol": "2pac_big_dag",     # the type of protocol
            "retry_delay": 5000,        # request block period
            "latency":[[0,19,64,55],[19,0,50,57],[64,50,0,28],[55,57,28,0]]
        }
    }
    try:
        LocalBench(bench_params, node_params).run(debug=True)
    except BenchError as e:
        Print.error(e)

@task
def create(ctx, nodes=2):
    ''' Create a testbed'''
    try:
        InstanceManager.make().create_instances(nodes)
    except BenchError as e:
        Print.error(e)

@task
def destroy(ctx):
    ''' Destroy the testbed '''
    try:
        InstanceManager.make().terminate_instances()
    except BenchError as e:
        Print.error(e)

@task
def cleansecurity(ctx):
    ''' Destroy the testbed '''
    try:
        InstanceManager.make().delete_security()
    except BenchError as e:
        Print.error(e)

@task
def start(ctx, max=10):
    ''' Start at most `max` machines per data center '''
    try:
        InstanceManager.make().start_instances(max)
    except BenchError as e:
        Print.error(e)

@task
def stop(ctx):
    ''' Stop all machines '''
    try:
        InstanceManager.make().stop_instances()
    except BenchError as e:
        Print.error(e)

@task
def install(ctx):
    try:
        Bench(ctx).install()
    except BenchError as e:
        Print.error(e)

@task
def uploadexec(ctx):
    try:
        Bench(ctx).upload_exec()
    except BenchError as e:
        Print.error(e)

@task
def info():
    ''' Display connect information about all the available machines '''
    try:
        InstanceManager.make().print_info()
    except BenchError as e:
        Print.error(e)

@task
def remote(ctx):
    ''' Run benchmarks on AWS '''
    bench_params = {
        'nodes': [10],
        'node_instance': 1,                                             # the number of running instance for a node  (max = 4)
        'duration': 30,
        'rate': 8000,                                                  # tx send rate
        'batch_size': [500,1000,2000,2500,3300,4000,5000],                              # the max number of tx that can be hold 
        'log_level': 0b1111,                                            # 0x1 infolevel 0x2 debuglevel 0x4 warnlevel 0x8 errorlevel
        'protocol_name': "bft/2pac",
        'runs': 1
    }
    node_params = {
        "pool": {
            # "rate": 1_000,              # ignore: tx send rate 
            "tx_size": 250,               # tx size
            # "batch_size": 200,          # ignore: the max number of tx that can be hold 
            "max_queue_size": 100000 
	    },
        "consensus": {
            "sync_timeout": 1000,      # node sync time
            "network_delay": 1000,     # network delay
            "min_block_delay": 0,       # send block delay
            "ddos": False,              # DDOS attack
            "faults": 3,                # the number of byzantine node
            "retry_delay": 5000        # request block period
        }
    }
    try:
        Bench(ctx).run(bench_params, node_params, debug=False)
    except BenchError as e:
        Print.error(e)

@task
def kill(ctx):
    ''' Stop any HotStuff execution on all machines '''
    try:
        Bench(ctx).kill()
    except BenchError as e:
        Print.error(e)

@task
def download(ctx,node_instance=2,ts="2024-06-19-09-02-46"):
    ''' download logs '''
    try:
        print(Bench(ctx).download(node_instance,ts).result())
    except BenchError as e:
        Print.error(e)

@task
def clean():
    cmd = f'{CommandMaker.cleanup_configs()};rm -f main'
    subprocess.run([cmd], shell=True, stderr=subprocess.DEVNULL)

@task
def logs():
    ''' Print a summary of the logs '''
    # try:
    print(LogParser.process('./logs/2024-11-23-07-07-38').result())
    # except ParseError as e:
    #     Print.error(BenchError('Failed to parse logs', e))

if __name__ == "__main__":
    task_name = sys.argv[1]
    if task_name not in tasks:
        print("not found this task")
    else:
        tasks[task_name]() 