# clip

A minimalist, self-hosted screenshot server written in Go. Helpful for sharing screenshots on services that don't
support media sharing like IRC or TeamSpeak.

## Deployment

Run it directly via Docker Compose using the GitHub Container Registry image.

```yaml
services:
  clip:
    image: docker.pkg.github.com/bemoty/clip/app:latest
    container_name: clip
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./uploads:/data
    environment:
      - PORT=:8080
      - STORAGE_PATH=/data
      - AUTH_KEY=your-secure-secret-key
      - BASE_URL=https://yourdomain.com
````

## Configuration

| Variable       | Description                       | Default               |
|:---------------|:----------------------------------|:----------------------|
| `PORT`         | The HTTP port to listen on        | `:8080`               |
| `STORAGE_PATH` | Directory where images are stored | `./data`              |
| `AUTH_KEY`     | Secret key for upload auth        | `no-auth`             |
| `BASE_URL`     | Public URL used to generate links | `http://i.bemoty.dev` |

## Usage

The server accepts a POST body at `/` and returns the URL to the uploaded image. The easiest way to use this is via
`curl`:

```console
curl -X POST -H "Authorization: Bearer <AUTH_KEY>" -F "file=@/path/to/image.png" <BASE_URL>/
```

## License

[MIT](./LICENSE)