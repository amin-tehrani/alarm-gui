# Maintainer: Your Name <your.email@example.com>
pkgname=alarm-app-git
pkgver=0.1.0.r0.g$(git rev-parse --short HEAD 2>/dev/null || echo "0000000")
pkgrel=1
pkgdesc="High-performance, CLI-driven, full-screen alarm application with video background"
arch=('x86_64')
url="https://github.com/yourusername/alarm-app"
license=('MIT')
depends=('glibc' 'ffmpeg' 'libglvnd' 'libxcursor' 'libxrandr' 'libxinerama' 'libxi')
makedepends=('go' 'git')
provides=('alarm-app')
conflicts=('alarm-app')
source=("git+${url}.git")
sha256sums=('SKIP')

pkgver() {
  cd "$srcdir/alarm-app"
  git describe --long --tags | sed 's/\([^-]*-g\)/r\1/;s/-/./g'
}

build() {
  cd "$srcdir/alarm-app"
  export CGO_CPPFLAGS="${CPPFLAGS}"
  export CGO_CFLAGS="${CFLAGS}"
  export CGO_CXXFLAGS="${CXXFLAGS}"
  export CGO_LDFLAGS="${LDFLAGS}"
  export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"
  go build -o alarm-app main.go
}

package() {
  cd "$srcdir/alarm-app"
  install -Dm755 alarm-app "$pkgdir/usr/bin/alarm-app"
  install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
  install -Dm644 README.md "$pkgdir/usr/share/doc/$pkgname/README.md"
}
