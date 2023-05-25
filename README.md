# Kinozal Monitor

Kinozal Monitor is a torrent management tool designed to interact with kinozal.tv and qbittorrent. It provides functionalities to add, remove, and get a list of torrents.

## Features

- Add a torrent by URL: This feature allows users to add a new torrent to the system by providing a URL.
- Remove a torrent by ID: Users can remove a torrent from the system by providing the ID of the torrent.
- Get a list of all torrents: Users can retrieve a list of all torrents that are currently in the system.

## Usage

The kinozal_monitor project provides a set of APIs to interact with the system.

- `GET /api/torrents`: Get a list of all torrents.
- `POST /api/add`: Add a torrent by URL. The URL is expected to be sent in the request body.
- `DELETE /api/remove`: Remove a torrent by ID. The ID is expected to be sent in the request body.

## Configuration

You need to configure the system before you can use it. The configuration includes setting up the username and password for kinozal.tv and qbittorrent. The configurations can be added to a config.ini file or set as environment variables.

Here's the structure of the config.ini file:

```[qbittorrent]
username = your_qbittorrent_username
password = your_qbittorrent_password
url = your_qbittorrent_url

[kinozal]
username = your_kinozal_username
password = your_kinozal_password
```

If the config.ini file is not found, the system will try to get the configurations from the environment variables:

- `QB_USERNAME`: Your qbittorrent username.
- `QB_PASSWORD`: Your qbittorrent password.
- `QB_URL`: Your qbittorrent URL.
- `KZ_USERNAME`: Your kinozal username.
- `KZ_PASSWORD`: Your kinozal password.

