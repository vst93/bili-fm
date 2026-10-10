# Avatar

`ui.Avatar` creates a picture of a person or a thing: its image, cropped to
a circle, or, without one, the initials of its name on a color picked from
the name, so that each person keeps theirs. It is a little more than twice
as high as the theme's font size.

```go
ui.Row(c).Gap(8).Children(func() {
	ui.Avatar(c, user.Name, user.Photo) // a *ui.Bitmap, or nil
	ui.Text(c, user.Name)
})
```

## Accessibility

Assistive technology sees an image named by the name.
