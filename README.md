# w2wesher

`w2wesher` creates and manages an encrypted mesh overlay network across a group of nodes using
* [wireguard](https://www.wireguard.com/) linux kernel module for communication;
* [libp2p](https://libp2p.io/) [pubsub](https://docs.libp2p.io/concepts/pubsub/overview/) for peer discovery, NAT traversal, and more.

Its main use-case is adding low-maintenance security to public-cloud networks or connecting different cloud providers.

## Security warning

**⚠ WARNING**: Unlike with the normal wireguard, you only need the following to access the network:
* Network `PSK`
* One peer ID and listen address

That effectively means that leaking `PSK` compromises your network. See [security considerations](#security-considerations) for more details.

## Quickstart

0. Before starting:
   1. make sure the [wireguard](https://www.wireguard.com/) kernel module is available on all nodes. It is bundled with linux newer than 5.6 and can otherwise be installed following the instructions [here](https://www.wireguard.com/install/).

   2. The following ports must be accessible between all nodes (see [configuration options](#configuration-options) to change these):
      - 10042 TCP (for peering, might be changed to UDP and QUIC)
      - 10042 UDP (for wireguard)

TODO: intsallation and configuration manual

### Permissions 

Note that `w2wesher` should never be started from a root user. Don't do that ever: there's a lot of third-party and potentially bugged code, which will listen to the whole Internet.

It is required to give the `wesher` binary enough capabilities to manage the `wireguard` interface via:
```
# setcap cap_net_admin=eip wesher
```

### (optional) systemd integration

TODO

## Installing from source

TODO

## Features

The `w2wesher` tool builds a cluster and manages the configuration of wireguard on each node to create peer-to-peer
connections between all nodes, thus forming a full mesh VPN.
This approach may not scale for hundreds of nodes (benchmarks accepted 😉), but is sufficiently performant to join
several nodes across multiple cloud providers, or simply to secure inter-node comunication in a single public-cloud.

### Automatic Key management

TODO

### Automatic IP address management

TODO rewrite the logic, write documentation

### mDNS dscovery (instead of original /etc/hosts modification)

TODO not implemented yet

### Seamless restarts

TODO not implemented yet

## Configuration options

TODO

## Running multiple clusters

TODO

## Security considerations

TODO

## Current known limitations

### Overlay IP collisions

TODO: will be engineered out

### Split-brain

TODO
