# Maintainer: kndxhz <kndxhz@163.com>
pkgname=waybar-mpris-lyrics
pkgver=1.0.0.r0.gGITCOMMIT
pkgrel=1
pkgdesc="A Waybar custom module that displays synchronized lyrics from MPRIS players"
arch=('x86_64')
url="https://github.com/kndxhz/waybar-MPRIS-lyrics"
license=('MIT')
depends=('glibc')
makedepends=('go' 'git')
optdepends=('playerctl: For media control support')
provides=("${pkgname%}")
conflicts=("${pkgname%}")
source=("git+${url}.git")
sha256sums=('SKIP')

pkgver() {
    cd "$srcdir/${pkgname%}"
    git describe --long --tags 2>/dev/null | sed 's/\([^-]*-g\)/r\1/;s/-/./g' || \
    printf "r%s.%s" "$(git rev-list --count HEAD)" "$(git rev-parse --short HEAD)"
}

build() {
    cd "$srcdir/${pkgname%}"
    export CGO_CPPFLAGS="${CPPFLAGS}"
    export CGO_CFLAGS="${CFLAGS}"
    export CGO_CXXFLAGS="${CXXFLAGS}"
    export CGO_LDFLAGS="${LDFLAGS}"
    export GOFLAGS="-buildmode=pie -trimpath -ldflags=-linkmode=external -mod=readonly -modcacherw"
    go build -o waybar-mpris-lyrics .
}

package() {
    cd "$srcdir/${pkgname%}"
    install -Dm755 waybar-mpris-lyrics "$pkgdir/usr/bin/waybar-mpris-lyrics"
    install -Dm644 README.md "$pkgdir/usr/share/doc/$pkgname/README.md"
    if [ -f LICENSE ]; then
        install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
    fi
}
