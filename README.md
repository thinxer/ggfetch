ggfetch
=======

A simple HTML fetcher based on groupcache, with TTL support.

Configuration Example
---------------------

```YAML
listen: 127.0.0.1:9001
me: 127.0.0.1:8001
peers:
    - 127.0.0.1:8001
    - 127.0.0.1:8002
# in MB
cache_size: 64
# in KB
max_item_size: 256
```

Usage
------

Call `/fetch?url={url}&ttl={ttl}`. TTL is specified in seconds, and the object is guaranteed to exist at most that duration. It may live shorter, though.

License
-------

MIT License. See LICENSE for details.
