# clip

A minimalist, self-hosted file upload and paste server written in Go. Helpful for sharing screenshots, code snippets,
and other files on services that don't support media sharing like IRC or TeamSpeak.

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
      - AUTH_KEY=your-secure-secret-key
      - BASE_URL=https://yourdomain.com
````

## Configuration

| Variable       | Description                                                                                                                       | Default               |
|:---------------|:----------------------------------------------------------------------------------------------------------------------------------|:----------------------|
| `PORT`         | The HTTP port to listen on                                                                                                        | `:8080`               |
| `STORAGE_PATH` | Directory where images are stored                                                                                                 | `./data`              |
| `AUTH_KEY`     | Secret key for upload auth                                                                                                        | `no-auth`             |
| `BASE_URL`     | Public URL used to generate links                                                                                                 | `http://i.bemoty.dev` |
| `MAX_FILE_MB`  | Maximum file upload size                                                                                                          | `100`                 |
| `PASTE_STYLE`  | The chroma style for code snippets; available styles can be found [here](https://github.com/alecthomas/chroma/tree/master/styles) | `dracula`             |

## Usage

The server accepts a POST body at `/` and returns the URL to the uploaded file. The easiest way to use this is via
`curl`:

```console
curl -X POST -H "Authorization: Bearer <AUTH_KEY>" -F "file=@/path/to/image.png" <BASE_URL>/
```

## License

[MIT](./LICENSE)