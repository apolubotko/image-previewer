# image-previewer

Final project

# Run example

1. Start the nginx by docker compose
``` docker-compose up -d ```

2. Start the service
``` make run ```

3. Make checks in browser

```bash

http://localhost:8081/fill/50/50/http://localhost:8088/img/gopher.jpg 
```

or

```bash
http://localhost:8081/fill/600/500/nas-national-prod.s3.amazonaws.com/apa_2015_harrycollins_275159_red-tailed_hawk_kk.jpg

```
