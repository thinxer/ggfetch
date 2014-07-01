ggfetch
=======

A simple HTML fetcher based on groupcache, with TTL support.

Usage
------

Call `/fetch?url={url}&ttl={ttl}`. TTL is specified in seconds, and the object is guaranteed to exist at most that duration. It may live shorter, though.

License
-------

MIT License. See LICENSE for details.
