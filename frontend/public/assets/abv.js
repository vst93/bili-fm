var table = "fZodR9XQDSUm21yCkr6zBqiveYah8bt4xsWpHnJE7jL5VG3guMTKNPAwcF";
var tr = {};
for (let i = 0; i < 58; i++) {
    tr[table[i]] = i;
}
var s = [11, 10, 3, 8, 4, 6];
var xor = 177451812,
    add = 8728348608;

function dec(x) {
    var r = 0;
    for (let i = 0; i < 6; i++) {
        r += tr[x[s[i]]] * Math.pow(58, i);
    }
    return (r - add) ^ xor;
}

function enc(x) {
    console.log(x)
    x = (x ^ xor) + add;
    var r = "BV1  4 1 7  ".split("");
    for (let i = 0; i < 6; i++) {
        r[s[i]] = table[Math.floor(x / Math.pow(58, i)) % 58];
    }
    return r.join("");
}