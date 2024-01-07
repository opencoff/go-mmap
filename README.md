# What is this

This is an OS independent `mmap(2)` abstractions.

# How can I use it?
Here is an example program:
```go

    // error checking omitted

    fd, _ := os.Open(filename)
    
    map := mmap.New(fd)

    // map the whole file as a RO mapping for sequential I/O
    p, err := map.Map(0, 0, mmap.PROT_READ, mmap.F_READAHEAD)
    if err != nil {
        ...
    }

    // p now represents the mapping of the entire file

    // buf represents the file contents as a byte slice
    buf := p.Bytes()
    ...

    p.Unmap()
    fd.Close()

```


