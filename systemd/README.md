# Boot persistence

The gateway is meant to run unattended on a Raspberry Pi and survive power cuts.
Two layers make that happen:

1. **Container restart policy** — every service in `docker-compose.yml` is
   `restart: unless-stopped`, so Docker restarts them automatically after a crash
   or a daemon restart.

2. **This systemd unit** — brings the whole stack up when the Pi boots (before
   any user logs in) and takes it down cleanly on shutdown. Enable Docker itself
   to start on boot too:

   ```bash
   sudo systemctl enable docker
   sudo cp systemd/zkbio.service /etc/systemd/system/zkbio.service
   # edit WorkingDirectory in the unit to your install path if not /opt/zkteco_bio_server
   sudo systemctl daemon-reload
   sudo systemctl enable --now zkbio.service
   ```

## Why nothing is lost on reboot

The local MariaDB database is the delivery queue. Every punch is written there
**before** it is forwarded, with a `delivery_status` of `Pending`, `Sent` or
`Failed`. On boot the gateway does not need to reconstruct any in-memory state —
it simply queries what is still `Pending` and resumes delivering it. A punch is
only marked `Sent` after the HR service returns a success for the batch that
carried it, so a crash mid-delivery at worst re-sends a batch (which HR
de-duplicates), never drops it.
