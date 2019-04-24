# tarsplitter

[Download latest release](https://github.com/AQUAOSOTech/tarsplitter/releases/latest)

Work with huge numbers of small files more quickly.

## Usage

### 1. Safely split a large tar archive into a specified number of smaller tar archives

- `i` - input tar file that you want to split, or '-' to read from stdin
- `o` - output pattern
- `p` - number of smaller archives to split the input archive into
- `s` - maximum size of each tar file, e.g. '100MB', '4G'

```bash
tarsplitter -m split -i archive.tar -o /tmp/archive-parts -p 4
```

```text
archive.tar is 529479680 bytes, splitting into 4 parts of 132369920 bytes
First new archive is /tmp/archive-parts0.tar
Processed files= 10000
Processed files= 20000
Processed files= 30000
Initialized next tar archive /tmp/archive-parts1.tar
Processed files= 40000
Processed files= 50000
Processed files= 60000
Initialized next tar archive /tmp/archive-parts2.tar
Processed files= 70000
Processed files= 80000
Processed files= 90000
Initialized next tar archive /tmp/archive-parts3.tar
Processed files= 100000
Processed files= 110000
Processed files= 120000
Done reading input archive
All done
```

If input is specified as '-', the archive will be read from stdin,
which allows piping directly from tar(1) to tarsplitter. This scenario
is only useful with the `-s` option to specify the maximum individual
part size.

For example:
```bash
tar -cvf - . | tarsplitter -i - -s 1G -o /tmp/archive-
```
This will create tar files that are at most 1GB in size, though
individual sizes will vary depending on the input files and how they're
sorted.


### 2. Multitheaded Archiving

Create a tar archive using `-p` concurrent threads.

- `i` - input file matching pattern
- `o` - output tar file path
- `p` - number of threads or files to read at once. `-p 10` will read up to 10 files from disk at a time

```
tarsplitter -m archive -i folder/*.json -o archive.tar
```

## Why

##### Splitting huge archives can be slow

It is possible to split large files, such as tar archives, into parts using the `split` utility. But you need to do a little work to precompute the byte split if you want a specific number of sub-files.

```
split -b 100m archive.tar
```

Worse, `split` won't keep all the files intact. Files will be split on the line, right near byte split, span archives, possibly making the archive unusable.

`tarsplitter` will not leave any broken files between the split archives.

##### Archiving lots of small files can be slow

It can be very slow to archive millions of small files. The `tarsplitter -m archive` mode will use as many cores as you want to create a tar archive, rather than the single threaded regular `tar` command.

## Contributors

- [AQUAOSO Technologies, PBC](https://aquaoso.com)
- [Jeff Parrish](https://github.com/ruffrey)

## MIT License

See the LICENSE file in this repository.
