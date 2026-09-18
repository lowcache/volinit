help:
	@echo "make serve       Live preview incl. drafts (http://localhost:1313)"
	@echo "make build       Production build to ./public (runs ./build.sh)"
	@echo "make deploy      Build, upload ./public to Cloudflare Workers, then verify"

serve:
	hugo server -D
