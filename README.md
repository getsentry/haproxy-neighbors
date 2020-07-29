# haproxy-neighbors

## Problems this solves
When running a stateful datastore, it's sometimes desirable to maintain
an http level consistent hash between peers amongst the cluster.

This is a challenge when deploying within contexts such as kuberenetes
where IPs and hosts move around.

Traditionally for us, we've had smarter clients which did the consistent
hashing to all nodes within the cluster, and all clients were required
to have the same view of the world with the same algorithms.

This requirement is done in the wrong place and puts extra burden on
the clients to do the correct thing.

## What this thing does
This moves the burden from the client to do consistent hashing to the
service itself.

`haproxy-neighbors` is indended to work as a sidecar, or a frontend for
the datastore. `haproxy-neighbors` uses SRV DNS records to discover its
peers, and configures it's own consistent hashing using haproxy amongst
the peers it has found.

This pattern suits well for any stateful store that is accessible via
SRV records, and allows the clients to be agnostic and unaware that this
behavior even happens.

Clients are able to make a request to any node within the cluster on the
frontend port, and requests are forwarded along to either the local peer
or a neighbor depending on the hashing required. This adds potentially
an extra hop, but shifts the responsibily into the service so clients
can be simpler.

## Configuration

All configuration is through environment variables, and this is expected
to run from its Docker container so it can own the haproxy subprocess.

```
HAPROXY_ENABLE_LOGS=false  # Enable http logging or not
HAPROXY_BALANCE=uri  # haproxy's 'balance' option
HAPROXY_HTTP_CHECK='meth OPTIONS'  # The http health check to use
HAPROXY_BIND=0.0.0.0:8888  # What port to bind haproxy on, this is the frontend
HAPROXY_THREADS=1  # If you want more than 1 thread for haproxy
HAPROXY_SLOTS=10  # The max number of slots to be allocated for hosts. This number must be higher than the number of hosts within your cluster, otherwise, hosts will just be ignored.
HAPROXY_STATS_BIND=  # Add an ip:port here if you'd like to enable the haproxy admin UI
HAPROXY_HEALTH_BIND=  # Add an ip:port here if you'd like to enable a health check endpoint in haproxy directly
HAPROXY_MAXCONN=500  # How many max connections haproxy can handle
HAPROXY_TIMEOUT_CONNECT=100ms  # haproxy's 'timeout connect' option
HAPROXY_TIMEOUT_CLIENT=5s  # haproxy's 'timeout client' option
HAPROXY_TIMEOUT_SERVER=5s  # haproxy's 'timeout server' option
HAPROXY_TIMEOUT_CHECK=100ms  # haproxy's 'timeout check' option

DISCOVERY_METHOD=dns  # currently required, but this is the only option. This is to future proof.
DISCOVERY_DNS_REFRESH=5s  # how often to query DNS record
DISCOVERY_DNS_NAME=  # The SRV DNS record to use for peers. This must be the form of `_[service]._[proto].[name].`
```
