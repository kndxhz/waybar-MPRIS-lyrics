# Maintainer: kndxhz <kndxhz@163.com>
pkgname=waybar-mpris-lyrics-git
pkgver=r14.d86247d
pkgrel=1
pkgdesc="A Waybar custom module that displays synchronized lyrics from MPRIS players"
arch=('x86_64')
url="https://github.com/kndxhz/waybar-MPRIS-lyrics"
license=('GPL3')
depends=('glibc')
makedepends=('go' 'git')
optdepends=('playerctl: For media control support')
provides=('waybar-mpris-lyrics')
conflicts=('waybar-mpris-lyrics')
source=("${pkgname%-git}::git+${url}.git")
sha256sums=('SKIP')

pkgver() {
    cd "$srcdir/${pkgname%-git}"
    printf "r%s.%s" "$(git rev-list --count HEAD)" "$(git rev-parse --short=7 HEAD)"
}

build() {
    cd "$srcdir/${pkgname%-git}"
    export CGO_CPPFLAGS="${CPPFLAGS}"
    export CGO_CFLAGS="${CFLAGS}"
    export CGO_CXXFLAGS="${CXXFLAGS}"
    export CGO_LDFLAGS="${LDFLAGS}"
    export GOFLAGS="-buildmode=pie -trimpath -ldflags=-linkmode=external -mod=readonly -modcacherw"

    go build -o waybar-mpris-lyrics .
}

package() {
    cd "$srcdir/${pkgname%-git}"
    install -Dm755 waybar-mpris-lyrics "$pkgdir/usr/bin/waybar-mpris-lyrics"
    install -Dm644 README.md "$pkgdir/usr/share/doc/$pkgname/README.md"
    if [ -f LICENSE ]; then
        install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
    fi
}
