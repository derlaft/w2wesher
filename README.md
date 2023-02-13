# w2wesher

`w2wesher` creates and manages an encrypted mesh overlay network across a group of nodes using
* [wireguard](https://www.wireguard.com/) linux kernel module for communication;
* [libp2p](https://libp2p.io/) [pubsub](https://docs.libp2p.io/concepts/pubsub/overview/) for peer discovery, NAT traversal, and more.

Its main use-case is adding low-maintenance security to public-cloud networks or connecting different cloud providers.

## Design goals

* Should use `libp2p` as it is the ultimate tool for building mesh networks:
	- See also: [hyprspace/hyprspace](https://github.com/hyprspace/hyprspace) for a libp2p solution working completely in userspace;
* Adding new nodes should be as simple as possible:
	- Just like the original project, it's way too complicated to manage so many configuration files. Joining the network should be as simple as possible;
* Focus on performance and powersave:
	- Using kernel-level wireguard. Userspace VPNs are way too slow and consume too much of your precious laptop battery;
	- Trying to minimize background tasks as much as possibl;
	- Trying to keep everything as simple as possible;
* KISS:
	- One configuration file.
	- Most settings should work in their default state.
* Complete isolation:
	- Avoiding DHT discovery

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

## TODO list
- [ ] Automatic key management.
- [ ] Rewrite automating IP management.
- [ ] Try to use mDNS discovery instead of original `/etc/hosts` modification.
- [ ] Seamless restarts: dump network state to the file.
- [ ] Periodic reconnect to disconnected nodes.
- [ ] Configuration tool/interface.
- [ ] Document initial setup.
- [ ] Document security.
- [ ] Document installation.
- [ ] Write a secure systemd unit.
- [ ] Test split-brain.
- [ ] Test roaming.

