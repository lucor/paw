
VERSION ?= 0.15.0

build:
	go build -ldflags "-w -s" -o ./bin/paw ./cmd/paw
	go build -ldflags "-w -s" -o ./bin/paw-cli ./cmd/paw-cli

darwin:
	fyne-cross darwin --arch="*" ./cmd/paw

freebsd:
	fyne-cross freebsd --arch="*" ./cmd/paw


LINUX_ARCH = amd64 arm64 arm
linux:
	@for arch in $(LINUX_ARCH); \
	do \
		echo "Packaging $$arch"; \
		fyne-cross linux --arch=$$arch -ldflags "-X main.Version=${VERSION}" ./cmd/paw; \
		golicense fyne-cross/bin/linux-$$arch/paw; \
		xz -d fyne-cross/dist/linux-$$arch/paw.tar.xz; \
		tar -rf fyne-cross/dist/linux-$$arch/paw.tar third-party-licenses.md LICENSE; \
		gzip -c fyne-cross/dist/linux-$$arch/paw.tar > dist/paw-${VERSION}-linux-$$arch.tar.gz; \
		rm third-party-licenses.md; \
	done

WINDOWS_ARCH = amd64
windows:
	@for arch in $(WINDOWS_ARCH); \
	do \
		echo "Packaging $$arch"; \
		#fyne-cross linux --arch=$$arch -ldflags "-X main.Version=${VERSION}" ./cmd/paw; \
		golicense fyne-cross/bin/windows-$$arch/paw.exe; \
		cp fyne-cross/dist/windows-$$arch/paw.exe.zip dist/paw-${VERSION}-windows-$$arch.zip; \
		zip dist/paw-${VERSION}-windows-$$arch.zip third-party-licenses.md LICENSE; \
		rm third-party-licenses.md; \
	done
