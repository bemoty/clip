# clip

A minimalist, self-hostable file upload and paste server written in Go. Helpful for sharing screenshots, code snippets,
and other files on services that don't support media sharing like IRC or TeamSpeak.

## Client

### Installation

**macOS**

```sh
brew tap bemoty/tap
brew install bemoty/tap/clip
```

**Windows**

```sh
scoop bucket add bemoty https://github.com/bemoty/scoop-bucket
scoop install clip
```

> **Note:** On Windows the binary is named `clipc` to avoid clashing with the built-in `clip.exe`. Use `clipc` in place
> of `clip` in all commands below.

**Arch Linux**

```sh
yay -S clip-bin
```

**From source**

Requires Go 1.26+.

```sh
go install github.com/bemoty/clip/cmd/client@latest
```

### Usage

```
clip [file] [flags]
```

With no arguments, clip reads from the clipboard. If stdin is piped, it reads from stdin instead. A file path can be
passed as an argument to upload a specific file. Pass `-` as the file argument to explicitly read from stdin.

```sh
clip                        # upload clipboard contents
clip note.txt               # upload a file
clip note.txt --ttl 7d      # upload with a time to live
cat main.go | clip -l go    # upload stdin with language hint
clip -c                     # uploads clipboard contents and copies the link to clipboard (recommended for hotkeys)
clip -                      # explicitly read from stdin
```

| Flag     | Short | Description                                     |
|:---------|:------|:------------------------------------------------|
| `--url`  |       | Server URL (overrides config)                   |
| `--key`  |       | Auth key (overrides config)                     |
| `--lang` | `-l`  | Language hint for syntax highlighting           |
| `--ttl`  |       | Time to live (e.g. `7d`, `1h`)                  |
| `--copy` | `-c`  | Immediately copy the returned link to clipboard |

### KDE Dolphin integration

If you are using KDE Plasma or KDE Dolphin, you can use the `clip` command as a right-click action in Dolphin to quickly
upload files.
To do this, create `~/.local/share/kio/servicemenus/clip.desktop`, which will add a right-click "Upload with clip"
action in Dolphin:

```desktop
[Desktop Entry]
Type=Service
MimeType=application/octet-stream;
Actions=Upload
X-KDE-ServiceTypes=KonqPopupMenu/Plugin

[Desktop Action Upload]
Name=Upload with clip
Icon=edit-copy
Exec=clip -c %F
```

Then make it executable:

```sh
chmod +x ~/.local/share/kio/servicemenus/clip.desktop
```

For more information on this feature,
read [KDE documentation](https://develop.kde.org/docs/apps/dolphin/service-menus/).

### Shell completions

```sh
clip completion bash > /etc/bash_completion.d/clip
clip completion zsh > "${fpath[1]}/_clip"
clip completion fish > ~/.config/fish/completions/clip.fish
```

### Configuration

On first use, run `clip config init` to create a config file. Open it with `clip config open`.

| Key   | Description             |
|:------|:------------------------|
| `url` | Server URL to upload to |
| `key` | Auth key for the server |

Configuration can also be provided via environment variables: `CLIP_URL`, `CLIP_KEY`, `CLIP_TTL`.

## Server

### Deployment

Run via Docker Compose using the GitHub Container Registry image.

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
```

### Building from source

Requires Go 1.26+.

```sh
git clone https://github.com/bemoty/clip
go build -o server ./cmd/server
./server
```

Or build a Docker image locally:

```sh
docker build -t clip .
```

### Configuration

| Variable         | Description                                                                                                | Default                |
|:-----------------|:-----------------------------------------------------------------------------------------------------------|:-----------------------|
| `PORT`           | HTTP port to listen on                                                                                     | `:8080`                |
| `STORAGE_PATH`   | Directory where files are stored                                                                           | `./data`               |
| `AUTH_KEY`       | Secret key for upload auth                                                                                 | `no-auth`              |
| `BASE_URL`       | Public URL used to generate links                                                                          | `https://i.bemoty.dev` |
| `MAX_FILE_MB`    | Maximum file upload size in MB                                                                             | `100`                  |
| `PASTE_STYLE`    | Chroma style for code pastes — [available styles](https://github.com/alecthomas/chroma/tree/master/styles) | `dracula`              |
| `DEFAULT_TTL`    | The default time to live for uploads                                                                       |                        |
| `SWEEP_INTERVAL` | Interval duration between file cleanup sweeps for uploads with TTL                                         | `1h`                   |

## License

[MIT](./LICENSE)
