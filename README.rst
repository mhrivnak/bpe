bpe
===

Batched Puppet Executor

This is meant to run as an agent on systems where an external client needs to
induce a run of puppet. One or more clients may request a puppet run in a
short amount of time, so this app batches those requests together as best it
can given how long any particular client is willing to wait.

Use Cases
---------

A client connects via http to either the path ``/sync`` or ``/async``,
depending on whether it wants the call to block until a run of puppet
completes.

When a client makes a request, the agent will ensure that a new run of puppet
starts at some point after the time of the request.

The client must specify the query parameter ``delay``, whose value is the
number of seconds the client is willing to wait until a run of puppet begins.
The agent will do its best to start a run within that time, but cannot
guarantee it.
