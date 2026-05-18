# suika

Self-hosted manga reader for CBZ/CBR archives. No database, reads directly from the filesystem.

## Features

- CBZ and CBR support
- ComicInfo.xml metadata
- JWT-based auth with a single shared password
- Multiple libraries via config
- In-memory page cache

## Build

```sh
go build -o suika ./cmd/suika
```

Or with Docker:

```sh
docker build -t suika .
```

## Run

Edit `config/config.yml` to set the UI password, JWT secret and library paths, then:

```sh
./suika -config config/config.yml
```

The UI is served at `http://localhost:8080`.

## Config

```yaml
app:
  port: 8080
  ui_password: "changeme"
  jwt_secret: "change-me-in-production"

libraries:
  - friendly_name: "Manga"
    path: "./manga"
```

Each entry under `libraries` is scanned recursively for `.cbz` and `.cbr` files.

## Credits

The reader frontend uses [manga-viewer.js](https://github.com/tokagemushi999/manga-viewer) by tokagemushi999.

