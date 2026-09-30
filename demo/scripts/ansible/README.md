# scripts/ansible/: discovery orchestration

The Ansible configuration that `demo.sh` drives. The controller (pqcota-ctl) **connects to each target node over SSH → runs the collector → retrieves the result JSON**. It is copied to `/work/ansible` in the controller image at build time.

| File | Role |
|---|---|
| [`discover.yml`](https://github.com/randyinthedev-hash/pqcota-discovery/blob/main/ansible/discover.yml) | **The reference playbook** (it is not in this folder: the original is in `ansible/` of `pqcota-discovery` and is copied here when the image is built). Four steps: ship the collector → run → retrieve → clean up. The JVM add-on is shipped conditionally, based on the `--recon` reconnaissance result. Porting to a real environment only means changing `collector_bin_dir` |
| `groups.ini` | **Group membership and the traffic scenario only** (not connection information). A per-node `traffic=` (the handshake targets that fill the observation window).<br>**Generated**: `topogen` builds it from `demo/topology/topology.yaml` and puts it in `demo/.generated/` (do not edit it by hand) |
| `targets.ini` | **Generated (not committed).** `pqcota-hosts` builds it from the user's `hosts.csv`. `[targets]` carries `ansible_host`, `ansible_user` and `ansible_ssh_private_key_file` (**connection secrets: runtime-only and never persisted**) |
| `ansible.cfg` | Host key and SSH options only. **No default connection user or key.** They always come from `targets.ini` (the output of pqcota-hosts) |

Discovery runs with the two inventories **merged**: `ansible-playbook -i targets.ini -i groups.ini discover.yml`. Connection (identity and secrets) belongs to `targets.ini`, and the scenario (traffic and groups) belongs to `groups.ini`, so the two lanes stay separate.

## To run the demo against your own hosts: edit `hosts.csv` (not the pqcota inventory)

The entry point for switching to real hosts is the **user-managed hosts file**:
- Replace the IPs, users and keys in `hosts.csv` (header `node_id,name,ip,port,ssh_user,ssh_key`) with your real infrastructure → `pqcota-hosts` produces `targets.ini` (with connection secrets, runtime-only) and an inventory upsert of the endpoints (secrets excluded).
- `traffic="pqc:host:port ssl:host:port ssh:host:port"` in `groups.ini`: the handshake targets to generate during the observation window (use `traffic=""` if there are none).
- **Use plain names for node IDs** (`web-gw`). No prefixes such as `node://...`, because the inventory comparison must match the scope master's node ID exactly.
- **Access secrets (keys and accounts) live only in `hosts.csv` (the user's file)**: they are never loaded into the pqcota inventory. The user points discovery at this file on every run.

## Known requirements (gotchas)

- **jvmscan environment**: querying the JCA provider chain needs `JAVA_BIN` (the real java path) and `JVMSCAN_CP` (the provider JAR, such as bcprov) for the provider to show up in the chain. The demo injects them from the playbook. On real hosts, set them to that node's actual paths.
- **SSH must be reachable**: the controller-to-target key distribution has to happen first (in the demo, `up.sh` handles it).
- **netcap permission**: observation needs `CAP_NET_RAW` (handshake plaintext only, no decryption).
