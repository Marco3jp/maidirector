.PHONY: build dev dev-frontend dev-backend clean

build: build-frontend build-backend

build-frontend:
	cd web && npm run build
	rm -rf cmd/maidirector/dist
	cp -r web/dist cmd/maidirector/dist

build-backend: build-frontend
	go build -o bin/maidirector ./cmd/maidirector

dev-frontend:
	cd web && npm run dev

dev-backend:
	go run ./cmd/maidirector serve

clean:
	rm -rf bin/ cmd/maidirector/dist web/dist
