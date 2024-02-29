#!/usr/bin/env zsh
repo_root=$(git rev-parse --show-toplevel)
set -x
host=$1

ssh "$host" bash -s <<'ENDSSH'
# Install Go
go_installed=$(grep -qxF 'export PATH=$PATH:/usr/local/go/bin' ~/.bashrc && echo "yes" || echo "no")
if [ "$go_installed" = "no" ]; then
  # Install Go
  curl -JLo  go1.22.0.linux-amd64.tar.gz https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
  rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
  echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
  rm go1.22.0.linux-amd64.tar.gz
fi

sqlite3_installed=$(which sqlite3 && echo "yes" || echo "no")

if [ "$sqlite3_installed" = "no" ]; then
  sudo apt-get update
  sudo apt-get install sqlite3
fi

username="verifiedPetition"

if id -u "$username" >/dev/null 2>&1; then
    echo "User exists"
else
    echo "User does not exist"
    sudo useradd -m "$username"
fi
ENDSSH

ssh "$host" bash -s <<'ENDSSH'
systemctl stop verifiedPetition
systemctl disable verifiedPetition
ENDSSH

bash "$repo_root"/build.sh
scp "$repo_root"/build/verifiedPetition "$host":/opt/bin/verifiedPetition
scp "$repo_root"/verifiedPetition.service "$host":/etc/systemd/system/verifiedPetition.service
scp "$repo_root"/.env "$host":/opt/bin/.env

ssh "$host" bash -s <<'ENDSSH'
systemctl enable verifiedPetition
systemctl start verifiedPetition
ENDSSH
