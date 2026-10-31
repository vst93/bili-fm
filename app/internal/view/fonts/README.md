# 内置字体（Inter v4.1 子集）

`Inter-*.ttf` 是从 [Inter v4.1](https://github.com/rsms/inter) 的 `extras/ttf/`
静态字重里裁出来的拉丁子集，由本包的 `fonts.go` 通过 `ui.RegisterFont` 注册成
`Inter` 族（`app/internal/view/font.go` 的字体栈引用它）。许可证见 `LICENSE.txt`（SIL OFL 1.1），
随包分发需保留它。

- 保留范围：Basic Latin / Latin-1 / Latin Extended-A、B、IPA、组合附加符号、
  常用标点、货币、字母式符号、箭头、数学符号、几何图形、杂项符号、BOM。
- 保留全部 OpenType 特性（`--layout-features='*'`），`tnum`（等宽数字）等
  本仓库用到的特性仍然有效。
- 没有覆盖的字符（中文、日文、西里尔、emoji…）由 `fontStack` 里后面的字体
  或系统回退接管，不会显示成豆腐块。

重新生成（需要 `pip install fonttools`）：

```bash
UNI="U+0000-024F,U+02B0-02FF,U+0300-036F,U+2000-206F,U+20A0-20CF,U+2100-214F,U+2190-21FF,U+2200-22FF,U+25A0-25FF,U+2600-26FF,U+FEFF"
for W in Regular Medium SemiBold Bold ExtraBold; do
  pyftsubset "Inter-$W.ttf" --unicodes="$UNI" --layout-features='*' \
    --glyph-names --symbol-cmap --legacy-cmap --notdef-glyph --notdef-outline \
    --recommended-glyphs --name-IDs='*' --name-legacy --name-languages='*' \
    --output-file="Inter-$W.ttf"
done
```

字重对应：400 Regular、500 Medium、600 SemiBold、700 Bold、800 ExtraBold。
代码里用到的 650 由 600 就近匹配（框架按字重差打分选面）。
