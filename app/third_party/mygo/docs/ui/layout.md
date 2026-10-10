# Layout

Elements lay out their children with flexbox, as in CSS, or in a
[grid](grid.md), in device-independent pixels (DIPs):

```go
ui.Row(c).Gap(8).Padding(12).Children(func() {
	ui.Icon(c, folder)
	ui.Text(c, "Documents").Grow(1)
	ui.Text(c, "12 items").TextColor(c.Theme().TextMuted)
})
```

## Containers

- `ui.Column` stacks its children from top to bottom and stretches them to
  its width; `ui.Box` is a column too. `ui.Row` places them from left to
  right and centers them vertically. `Reverse` lays them out the other
  way, from the right or the bottom, as CSS's `row-reverse` and
  `column-reverse`.
- `ui.Spacer` takes the free space of its row or column, pushing its
  siblings apart, and `ui.Divider` draws a thin line across its row or
  column. `Dividers(width, color)` draws a line between each two children
  of a row, a column or a [list](list.md), in the middle of the gap
  between them, which it does not widen: give the element a `Gap` at
  least as wide.
- `ui.Grid` lays its children out in columns and rows: see
  [Grid](grid.md). Containers that scroll are [scroll views](scroll.md),
  and [lists](list.md) build only the rows in view.

## Sizes

- `Width`, `Height` and `Size` set sizes; `WidthPercent` and `HeightPercent`
  take a share of the parent; `MinWidth`, `MaxWidth`, `MinHeight` and
  `MaxHeight` bound them, and their `Percent` forms by a share of the
  parent; `Fill`, `FillWidth` and `FillHeight` take the parent's whole
  content box.
- `Grow(1)` gives an element the free space along its parent's direction,
  like `flex: 1`: a list that fills the rest of a window, or a `Spacer` that
  pushes the elements after it to the end. `Shrink`, `Basis` and
  `BasisPercent` work as in CSS.
- `AspectRatio` keeps an element's proportions.

## Spacing

`Gap` spaces the children, `GapX` and `GapY` horizontally and vertically
apart; `Padding` and `Margin` take one, two or four values, as in CSS, and
`PaddingX`, `PaddingY`, `MarginX` and `MarginY` two sides. A margin of
`ui.Auto` takes the free space on its side, as in CSS: `Margin(0, ui.Auto)`
centers an element in a column, and `Margin(0, 0, 0, ui.Auto)` sends a
row's child, and those after it, to the end.

## Alignment

`Justify` places the children along the direction (`Start`, `Center`,
`End`, `SpaceBetween`, `SpaceAround`, `SpaceEvenly`), `AlignItems` across
it (`Start`, `Center`, `End`, `Stretch`), `AlignSelf` one child; `Center`
centers them both ways. `Wrap` wraps a row onto more lines, which
`WrapReverse` stacks upward, and `AlignContent` places those lines (as
`Justify` places children, or `Stretch`).

## Positioning

`Absolute` takes an element out of the flow, placed with `Top`, `Right`,
`Bottom` and `Left` in its parent, in DIPs or with `TopPercent` and the
like; automatic margins center it between the sides it is placed from. As
in CSS, they are measured from just inside the parent's border, whatever
its padding: in a box with a border 1 DIP wide, `Top(0)` is 1 DIP below
the box's top edge. On an element in the flow, `Top` and the others move
it from where the layout put it, without moving its siblings, as CSS's
relative positioning does: a badge raised a little above its text.

```go
ui.Box(c).Size(40, 40).Children(func() {
	ui.Icon(c, bell).FontSize(24)
	ui.Badge(c, "3").Absolute().Top(-4).Right(-6)
})
```

`Attach(at, self)` takes an element out of the flow too, and puts a point
of it (`self`) on a point of its parent's box (`at`): a corner, the middle
of a side or the center, as `ui.AnchorTopRight` or `ui.AnchorCenter`. The
element keeps its own size, and `Top`, `Right`, `Bottom` and `Left` move
it from there:

```go
ui.Box(c).Children(func() {
	ui.Avatar(c, "Ada Lovelace", nil)
	// Centered on the avatar's top right corner, whatever its size.
	ui.Badge(c, "3").Attach(ui.AnchorTopRight, ui.AnchorCenter)
})
ui.Box(c).Fill().Children(func() {
	ui.PrimaryButton(c, "New").Attach(ui.AnchorBottomRight, ui.AnchorBottomRight).Right(16).Bottom(16)
})
```

## Clipping and visibility

- `Clip` cuts its children to its rounded box, inside its border, which it
  draws over them; `ClipX` and `ClipY` clip only on the sides or above and
  below.
- `Invisible` hides an element and its children, which keep their room
  but draw nothing and take neither the pointer nor the focus.
- `Debug` outlines an element and everything inside it, with their padding
  and margins, to see why the layout is what it is. The
  [inspector](inspector.md) shows the boxes of every element, and why.
- Elements move, resize and recolor smoothly with a
  [transition](transitions.md).
