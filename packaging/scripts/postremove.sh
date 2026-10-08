#!/bin/bash
systemctl daemon-reload

# dpkg --purge removes the configuration files the package created. The
# secret-store key stays, with the database it decrypts, which purge keeps.
if [ "$1" = "purge" ]; then
  for name in environment application.properties database.properties smtp.properties; do
    rm -f "/etc/sensor-hub/$name"
  done
fi
exit 0
