#!/bin/bash
cd ~/code/daleego-playlist/web
nvm use 24
npm run build
rsync -a --delete dist/ /var/www/daleego-playlist/
cd ../
go build -trimpath -o /tmp/daleego-playlist-server ./cmd/server
sudo install -m 0755 /tmp/daleego-playlist-server /usr/local/bin/daleego-playlist/server
sudo systemctl daemon-reload
sudo systemctl restart daleego-playlist-backend
