## System Operations
## :switch: ..........: Rebuild and switch system live
switch:
	sudo nixos-rebuild switch

## :build: ..........: Build without switching
build:
	nix build

## Anonymous Mode
## :anon-arm: ..........: Arm anonymous mode
anon-arm:
	@echo arming

## Blog Operations
## serve: -----: Development server at localhost
serve:
	hugo server
## deploy: -----: Deploy to production
deploy:
	hugo deploy
