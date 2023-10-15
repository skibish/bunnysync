# Bunny Sync

Tiny tool to sync local folder to [Bunny Edge Storage](https://bunny.net/pricing/storage/) using [HTTP API](https://docs.bunny.net/reference/storage-api).

> [!WARNING]
> Very experimental, not for production, no tests, use at your own risk.

## Motivation

I'm trying out Bunny for my website.
Uploading files to Edge Storage is not simple if you do not want to drag'n'drop files using UI.
What I am talking about:

- Attempt to use some FTP GitHub Actions failed due to weird behavior --- they just create a lot of nested empty folders and that's it.
- Using `curl` is possible but I do not want to re-upload all the files every time something is changed.
I want to upload only what was changed.
I also want to upload files in parallel.
Sounds complicated for bash and curl only solution.

That's why this tiny tool is built.

## Usage

```txt
Usage of bunnysync:
  -dry-run
    	dry run (performs no changes to remote)
  -src string
    	source path (default "/my/current/folder")
  -storage-api-key string
    	storage api key
  -storage-endpoint string
    	storage endpoint (default "storage.bunnycdn.com")
  -storage-zone string
    	storage zone name
```

Example

```bash
bunnysync \
    -src /public \
    -storage-zone $STORAGE_ZONE \
    -storage-api-key $STORAGE_API_KEY
```
