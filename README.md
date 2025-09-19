<p style="text-align:center;">
<img src="logo/logo.png" width="350" alt="Kinozal Monitor Logo">
</p>

# Kinozal Monitor

Kinozal Monitor is a torrent management tool designed to interact with kinozal.tv and qbittorrent. It provides functionalities to add, remove, and get a list of torrents, as well as real-time updates via WebSocket.

## Features

- Add a torrent by URL: Add a new torrent to the system by providing a URL.
- Remove a torrent by ID: Remove a torrent from the system by providing the ID of the torrent.
- Get a list of all torrents: Retrieve a list of all torrents currently in the system.
- Watch torrents: Set a watch flag for torrents to monitor them.
- Real-time updates: Receive real-time updates about torrent status via WebSocket.
- Download path selection: Choose from available download paths for your torrents.

## Usage

The Kinozal Monitor project provides a set of APIs to interact with the system:

- `GET /api/torrents`: Get a list of all torrents.
- `GET /api/download-paths`: Get a list of available download paths from qBittorrent.
- `POST /api/add`: Add a torrent by URL. The URL and optional download path are sent in the request body.
- `POST /api/watch`: Set a watch flag for a torrent. The URL and watch period are sent in the request body.
- `DELETE /api/remove`: Remove a torrent by ID. The ID is sent in the request body.
- `GET /ws`: WebSocket endpoint for real-time updates.

## Configuration

You need to configure the system before you can use it. The configuration includes setting up the username and password for kinozal.tv and qbittorrent. The configurations can be added to a config.ini file or set as environment variables.

Here's the structure of the config.ini file:

```ini
[qbittorrent]
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

## Initialization

Upon starting the application, the `TrackerManager` is initialized with configurations for interacting with different torrent trackers. It sets up a `KinozalTracker` instance if valid credentials are provided.

## Tracker Configuration

Each tracker, such as `KinozalTracker`, is configured with necessary credentials and URLs to interact with the respective service. Ensure that these configurations are correctly set in the `config.ini` file or through environment variables before running the application.

## Running with Docker

The project includes Docker support. You can use the provided `docker-compose.yml` file to run the application in a containerized environment:

```bash
docker-compose up -d
```

This will start the Kinozal Monitor service along with any required dependencies.
