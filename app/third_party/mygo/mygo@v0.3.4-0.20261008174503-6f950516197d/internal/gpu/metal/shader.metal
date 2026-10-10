// The one shader of the Metal renderer, as shader.hlsl is the Direct3D
// one: every scene op is an instanced quad, and the fragment shader
// computes the coverage of rounded rectangles, borders, gradients, stripes
// and shadows from signed distances, as the CPU renderer (internal/raster)
// does. Colors are straight (not premultiplied) and blending happens in
// sRGB space, as in browsers, with a second color for the source's alpha
// of each channel, which subpixel glyphs need. go generate compiles it
// into shaderlib.go. The libraries of effects (scene.Effect) take its
// start, with EFFECT defined, before effect.metal.

#include <metal_stdlib>
using namespace metal;

// One instance per op, as internal/gpu builds them.
struct Inst {
	float4 rect;      // x, y, width, height in pixels
	float4 radii;     // top-left, top-right, bottom-right, bottom-left
	float4 inner;     // radii of the border's inner edge, or of the box casting a shadow
	float4 color;
	float4 color2;    // gradient end
	float4 border;    // border color
	float4 grad;      // gradient start and end points, or stripes
	float4 uv;        // texture rectangle, normalized, border widths, or the box casting a shadow
	float4 clip;      // the innermost clip rectangle
	float4 clipRadii;
	float4 params;    // kind, dashed or grayscale, sigma or paint, opacity
};

// The color and, for dual-source blending, the source's alpha of each
// channel.
struct PSOut {
	float4 color [[color(0), index(0)]];
	float4 alpha [[color(0), index(1)]];
};

struct VSOut {
	float4 pos [[position]];
	float2 p;
	float2 tex;
	uint inst [[flat]];
};

// globals holds the frame's width and height in pixels, and 1 in z when
// its target keeps colors outside the sRGB gamut (extended sRGB, float16).
vertex VSOut vs(uint vid [[vertex_id]], uint iid [[instance_id]],
                const device Inst *insts [[buffer(0)]],
                constant float4 &globals [[buffer(1)]]) {
	Inst i = insts[iid];
	float2 corner = float2(float(vid & 1u), float(vid >> 1u));
	float4 r = i.rect;
	float kind = i.params.x;
	if (kind < 0.5f || kind > 5.5f) { // fills and effects
		r = float4(r.xy - 1.0f, r.zw + 2.0f);
	} else if (kind < 1.5f) {
		float e = 3.0f * i.params.z + 1.0f;
		r = float4(r.xy - e, r.zw + 2.0f * e);
	}
	float2 p = r.xy + corner * r.zw;
	VSOut o;
	o.pos = float4(p / globals.xy * float2(2.0f, -2.0f) + float2(-1.0f, 1.0f), 0.0f, 1.0f);
	o.p = p;
	o.tex = mix(i.uv.xy, i.uv.zw, corner);
	o.inst = iid;
	return o;
}

float sdRoundRect(float2 p, float4 rect, float4 radii) {
	float2 h = rect.zw * 0.5f;
	float2 q = p - rect.xy - h;
	float r = q.x < 0.0f ? (q.y < 0.0f ? radii.x : radii.w) : (q.y < 0.0f ? radii.y : radii.z);
	float2 a = abs(q) - h + r;
	return length(max(a, float2(0.0f))) + min(max(a.x, a.y), 0.0f) - r;
}

float coverage(float d) { return saturate(0.5f - d); }

// Continuous corners are Core Animation's, as internal/raster/corner.go
// explains: a corner of radius r whose curve leaves the edges e from it,
// blended toward a quarter circle by cx along its horizontal edge and cy
// along its vertical one, its curve's box ending at eff as clamped.
constant float contExtent = 1.528665f;

struct ContCorner {
	float r, e, eff, cx, cy;
};

// contCorner returns the corner of radius r of a rectangle w×h sharing
// its horizontal edge with a corner of radius rh, its vertical one with
// one of radius rv.
ContCorner contCorner(float r, float rh, float rv, float w, float h) {
	ContCorner c;
	c.r = r;
	c.e = contExtent * r;
	c.cx = saturate((contExtent - w / max(r + rh, 1e-6f)) / (contExtent - 1.0f));
	c.cy = saturate((contExtent - h / max(r + rv, 1e-6f)) / (contExtent - 1.0f));
	c.eff = c.e + (r - c.e) * max(c.cx, c.cy);
	return c;
}

// contDist returns the corner's value at the point u inside its vertical
// edge and v inside its horizontal one: about the signed distance to its
// edge near it.
float contDist(ContCorner c, float u, float v) {
	float2 a = max(0.0f, 1.0f - float2(u, v) / c.e);
	float l = length(a);
	float hi = max(a.x, a.y);
	float rho = hi > 0.0f ? min(min(a.x, a.y) / hi, 1.0f) : 0.0f;
	float p = (((-0.926054f * rho + 3.15601f) * rho - 3.64122f) * rho + 1.26803f) * rho + 0.268531f;
	float f1 = l + 1.0f - 1.0f / (1.0f - rho * rho * min(l, 1.0f) * p);
	float f2 = length(max(0.0f, a * contExtent - (contExtent - 1.0f))) * 0.654166f + 0.345834f;
	float s = a.y > a.x ? 1.0f : -1.0f;
	float w = saturate(0.5f - s + s * rho);
	float fx = f1 + (f2 - f1) * c.cx, fy = f1 + (f2 - f1) * c.cy;
	return min(max(c.eff - u, c.eff - v), 0.0f) + c.e * (fx + (fy - fx) * w - 1.0f);
}

// contInset returns how far inside its vertical edge the corner's edge is
// v inside its horizontal one, which lies between the quarter circles of
// radius r and e, by regula falsi (Illinois).
float contInset(ContCorner c, float v) {
	if (v >= c.e || c.r <= 0.0f) {
		return 0.0f;
	}
	float lo = v < c.r ? c.r - sqrt(v * (2.0f * c.r - v)) : 0.0f;
	float hi = c.e - sqrt(v * (2.0f * c.e - v));
	float dlo = contDist(c, lo, v), dhi = contDist(c, hi, v);
	if (dlo <= 0.0f) {
		return lo;
	}
	if (dhi >= 0.0f) {
		return hi;
	}
	int side = 0;
	for (int n = 0; n < 5; n++) {
		float m = (lo * dhi - hi * dlo) / (dhi - dlo);
		float dm = contDist(c, m, v);
		if (dm > 0.0f) {
			lo = m; dlo = dm;
			if (side == 1) {
				dhi *= 0.5f;
			}
			side = 1;
		} else {
			hi = m; dhi = dm;
			if (side == -1) {
				dlo *= 0.5f;
			}
			side = -1;
		}
	}
	return (lo * dhi - hi * dlo) / (dhi - dlo);
}

// areaCoverage returns the area of the pixel at p inside the rectangle,
// near square corners: exact for lines thinner than a pixel too.
float areaCoverage(float2 p, float4 rect) {
	float2 c = saturate(min(rect.xy + rect.zw, p + 0.5f) - max(rect.xy, p - 0.5f));
	return c.x * c.y;
}

// contValue returns the value of a rectangle with continuous corners c at
// p, the largest of its corners' and its signed distance, and whether a
// corner's curve reaches p.
float contValue(float2 p, float4 rect, thread const ContCorner *c, thread bool &curved) {
	float4 d = float4(p - rect.xy, rect.xy + rect.zw - p); // left, top, right, bottom
	float4 u = d.xzzx, v = d.yyww;
	float value = max(max(-d.x, -d.y), max(-d.z, -d.w));
	curved = false;
	for (int i = 0; i < 4; i++) {
		if (c[i].r > 0.0f && u[i] < c[i].e && v[i] < c[i].e) {
			value = max(value, contDist(c[i], u[i], v[i]));
			curved = true;
		}
	}
	return value;
}

// contCoverage is rectCoverage with continuous corners of radii r: away
// from their curves, the area of the pixel inside the rectangle; near
// them, the value over the sum of its derivatives, as Core Animation
// antialiases.
float contCoverage(float2 p, float4 rect, float4 r) {
	ContCorner c[4] = {
		contCorner(r.x, r.y, r.w, rect.z, rect.w), contCorner(r.y, r.x, r.z, rect.z, rect.w),
		contCorner(r.z, r.w, r.y, rect.z, rect.w), contCorner(r.w, r.z, r.x, rect.z, rect.w),
	};
	bool curved, ignored;
	float d = contValue(p, rect, c, curved);
	if (!curved) {
		return areaCoverage(p, rect);
	}
	if (abs(d) > 2.0f) {
		return d < 0.0f ? 1.0f : 0.0f;
	}
	const float h = 1.0f / 16.0f;
	float dx = contValue(p + float2(h, 0.0f), rect, c, ignored);
	float dy = contValue(p + float2(0.0f, h), rect, c, ignored);
	return saturate(0.5f - d * h / max(abs(dx - d) + abs(dy - d), 1e-6f));
}

// rectCoverage returns how much of the pixel at p a rounded rectangle
// covers: by the distance to its edge near rounded corners, and exactly,
// the area of the pixel inside it, near square ones. Negative radii are
// continuous corners.
float rectCoverage(float2 p, float4 rect, float4 radii) {
	if (any(radii < 0.0f)) {
		return contCoverage(p, rect, -radii);
	}
	float2 q = p - rect.xy - rect.zw * 0.5f;
	float r = q.x < 0.0f ? (q.y < 0.0f ? radii.x : radii.w) : (q.y < 0.0f ? radii.y : radii.z);
	if (r > 0.0f) {
		return coverage(sdRoundRect(p, rect, radii));
	}
	return areaCoverage(p, rect);
}

float4 premul(float4 c) { return float4(c.rgb * c.a, c.a); }

float3 unpremul(float4 c) { return c.a > 0.0f ? c.rgb / c.a : float3(0.0f); }

// What follows is the renderer's own; effects (effect.metal) take what is
// above.
#ifndef EFFECT

// textCoverage corrects the coverage a of a glyph of straight color c as
// Direct2D blends text (scene.TextCoverage): it enhances the contrast,
// the more the darker c is, and corrects for gamma with the ratios g.
float textCoverage(float a, float3 c, float contrast, float boost, float4 g) {
	float k = contrast * saturate(3.0f - 4.0f * dot(c, float3(0.30f, 0.59f, 0.11f))) + boost;
	a = a * (k + 1.0f) / (a * k + 1.0f);
	float f = dot(c, float3(0.25f, 0.5f, 0.25f));
	return saturate(a + a * (1.0f - a) * ((g.x * f + g.y) * a + (g.z * f + g.w)));
}

// subpixelCoverage does the same for each subpixel of a subpixel glyph.
float3 subpixelCoverage(float3 a, float3 c, float contrast, float boost, float4 g) {
	float k = contrast * saturate(3.0f - 4.0f * dot(c, float3(0.30f, 0.59f, 0.11f))) + boost;
	a = a * (k + 1.0f) / (a * k + 1.0f);
	return saturate(a + a * (1.0f - a) * ((g.x * c + g.y) * a + (g.z * c + g.w)));
}

// erf2 approximates the error function (Abramowitz and Stegun 7.1.27).
float2 erf2(float2 x) {
	float2 s = sign(x);
	float2 a = abs(x);
	x = 1.0f + (0.278393f + (0.230389f + 0.078108f * (a * a)) * a) * a;
	x *= x;
	return s - s / (x * x);
}

float gaussian(float x, float sigma) {
	return exp(-(x * x) / (2.0f * sigma * sigma)) / (2.50662827463f * sigma);
}

// The blurred rounded box of Evan Wallace: exact along x, four samples
// along y; cc is its continuous corner, if any.
float shadowX(float x, float y, float sigma, float corner, float2 h, bool continuous, ContCorner cc) {
	float curved;
	if (continuous) {
		float v = h.y - abs(y);
		curved = v < cc.e ? h.x - contInset(cc, v) : h.x;
	} else {
		float delta = min(h.y - corner - abs(y), 0.0f);
		curved = h.x - corner + sqrt(max(0.0f, corner * corner - delta * delta));
	}
	float2 integral = 0.5f + 0.5f * erf2((x + float2(-curved, curved)) * (sqrt(0.5f) / sigma));
	return integral.y - integral.x;
}

float boxShadow(float2 p, float4 rect, float sigma, float4 radii) {
	float4 r = abs(radii);
	float corner = max(max(r.x, r.y), max(r.z, r.w));
	bool continuous = any(radii < 0.0f) && corner > 0.0f;
	ContCorner cc = contCorner(corner, corner, corner, rect.z, rect.w);
	float2 h = rect.zw * 0.5f;
	p -= rect.xy + h;
	float low = p.y - h.y;
	float high = p.y + h.y;
	float from = clamp(-3.0f * sigma, low, high);
	float to = clamp(3.0f * sigma, low, high);
	float dy = (to - from) / 4.0f;
	float y = from + dy * 0.5f;
	float v = 0.0f;
	for (int n = 0; n < 4; n++) {
		v += shadowX(p.x, p.y - y, sigma, corner, h, continuous, cc) * gaussian(y, sigma) * dy;
		y += dy;
	}
	return v;
}

// toLinear and toSRGB mirror the curve around zero, as extended sRGB does,
// for the components of wide colors.
float3 toLinear(float3 c) {
	float3 a = abs(c);
	return sign(c) * select(pow((a + 0.055f) / 1.055f, float3(2.4f)), a / 12.92f, a <= 0.04045f);
}

float3 toSRGB(float3 c) {
	float3 a = abs(c);
	return sign(c) * select(1.055f * pow(a, float3(1.0f / 2.4f)) - 0.055f, a * 12.92f, a <= 0.0031308f);
}

float3 cbrt3(float3 v) { return sign(v) * pow(abs(v), float3(1.0f / 3.0f)); }

float3 oklab(float3 srgb) {
	float3 c = toLinear(srgb);
	float3 lms = cbrt3(float3(
		0.4122214708f * c.r + 0.5363325363f * c.g + 0.0514459929f * c.b,
		0.2119034982f * c.r + 0.6806995451f * c.g + 0.1073969566f * c.b,
		0.0883024619f * c.r + 0.2817188376f * c.g + 0.6299787005f * c.b));
	return float3(
		0.2104542553f * lms.x + 0.7936177850f * lms.y - 0.0040720468f * lms.z,
		1.9779984951f * lms.x - 2.4285922050f * lms.y + 0.4505937099f * lms.z,
		0.0259040371f * lms.x + 0.7827717662f * lms.y - 0.8086757660f * lms.z);
}

float3 fromOklab(float3 lab) {
	float3 lms = float3(
		lab.x + 0.3963377774f * lab.y + 0.2158037573f * lab.z,
		lab.x - 0.1055613458f * lab.y - 0.0638541728f * lab.z,
		lab.x - 0.0894841775f * lab.y - 1.2914855480f * lab.z);
	lms = lms * lms * lms;
	return toSRGB(float3(
		4.0767416621f * lms.x - 3.3077115913f * lms.y + 0.2309699292f * lms.z,
		-1.2684380046f * lms.x + 2.6097574011f * lms.y - 0.3413193965f * lms.z,
		-0.0041960863f * lms.x - 0.7034186147f * lms.y + 1.7076147010f * lms.z));
}

// paint returns the premultiplied color at p of plain color, a gradient
// mixed in sRGB (1) or Oklab (2), or stripes (3), as scene.Paint says. In a
// target that keeps colors outside the sRGB gamut (wide, extended sRGB), an
// Oklab gradient keeps those its mix has, which others clamp.
float4 paint(float2 p, float mode, float4 rect, float4 c1, float4 c2, float4 g, bool wide) {
	if (mode < 0.5f) {
		return premul(c1);
	}
	if (mode < 2.5f) {
		float2 d = g.zw - g.xy;
		float t = saturate(dot(p - g.xy, d) / max(dot(d, d), 0.0001f));
		float a = mix(c1.a, c2.a, t);
		if (mode < 1.5f) {
			return float4(mix(c1.rgb * c1.a, c2.rgb * c2.a, t), a);
		}
		float3 lab = mix(oklab(c1.rgb) * c1.a, oklab(c2.rgb) * c2.a, t);
		if (a <= 0.0f) {
			return float4(0.0f);
		}
		float3 rgb = fromOklab(lab / a);
		return float4((wide ? rgb : saturate(rgb)) * a, a);
	}
	float s = dot(p - rect.xy, g.xy);
	float phase = s - g.w * floor(s / g.w);
	float cov = coverage(min(max(-phase, phase - g.z), g.w - phase));
	return premul(c1) * cov + premul(c2) * (1.0f - cov);
}

// dash returns how much of a dashed border shows at p: each side, which
// the pixel belongs to when it is nearest that side's edge in widths of
// its border, has an odd number of dashes and gaps of equal length, about
// three widths, starting and ending with a dash.
float dash(float2 p, float4 rect, float4 w) {
	float2 q = p - rect.xy;
	float dt = w.x > 0.0f ? q.y / w.x : 1e9f;
	float dr = w.y > 0.0f ? (rect.z - q.x) / w.y : 1e9f;
	float db = w.z > 0.0f ? (rect.w - q.y) / w.z : 1e9f;
	float dl = w.w > 0.0f ? q.x / w.w : 1e9f;
	float s, len, bw;
	if (dt <= dr && dt <= db && dt <= dl) {
		s = q.x; len = rect.z; bw = w.x;
	} else if (dr <= db && dr <= dl) {
		s = q.y; len = rect.w; bw = w.y;
	} else if (db <= dl) {
		s = rect.z - q.x; len = rect.z; bw = w.z;
	} else {
		s = rect.w - q.y; len = rect.w; bw = w.w;
	}
	// n - 1 is how many periods of a dash and a gap, six widths, fit in
	// len: GPUs divide less exactly than CPUs, and may count one too few
	// or too many where len is a whole number of periods, as sides often
	// are, so the count is checked by multiplying back.
	float n = max(1.0f, floor((len / (3.0f * bw) + 1.0f) * 0.5f + 0.5f));
	float period = 6.0f * bw;
	n += n * period <= len ? 1.0f : 0.0f;
	n -= (n - 1.0f) * period > len ? 1.0f : 0.0f;
	float seg = len / (2.0f * n - 1.0f);
	float k = floor(s / seg);
	float f = s - k * seg;
	float edge = min(f, seg - f);
	return coverage(k - 2.0f * floor(k * 0.5f) < 0.5f ? -edge : edge);
}

fragment PSOut ps(VSOut v [[stage_in]],
                   const device Inst *insts [[buffer(0)]],
                   constant float4 &globals [[buffer(1)]],
                   texture2d<float> maskTex [[texture(0)]],
                   texture2d<float> colorTex [[texture(1)]],
                   texture2d<float> imageTex [[texture(2)]],
                   sampler samp [[sampler(0)]]) {
	Inst i = insts[v.inst];
	float kind = i.params.x;
	bool wide = globals.z > 0.5f;
	float4 res;
	if (kind < 0.5f) {
		float outer = rectCoverage(v.p, i.rect, i.radii);
		res = paint(v.p, i.params.z, i.rect, i.color, i.color2, i.grad, wide) * outer;
		float4 bw = i.uv; // top, right, bottom, left
		if (any(bw > 0.0f)) {
			float4 ir = float4(i.rect.xy + bw.wx, i.rect.zw - bw.yz - bw.wx);
			float innerCov = (ir.z > 0.0f && ir.w > 0.0f) ? rectCoverage(v.p, ir, i.inner) : 0.0f;
			float bc = saturate(outer - innerCov);
			if (i.params.y > 0.5f) {
				bc *= dash(v.p, i.rect, bw);
			}
			float4 b = premul(i.border) * bc;
			res = b + res * (1.0f - b.a);
		}
	} else if (kind < 1.5f) {
		float sigma = i.params.z;
		float s = sigma > 0.0f ? boxShadow(v.p, i.rect, sigma, i.radii) : rectCoverage(v.p, i.rect, i.radii);
		if (i.uv.z > 0.0f && i.uv.w > 0.0f) {
			s *= 1.0f - rectCoverage(v.p, i.uv, i.inner); // outside the box casting it
		}
		res = premul(i.color) * s;
	} else if (kind < 2.5f) {
		float4 c = paint(v.p, i.params.z, i.rect, i.color, i.color2, i.grad, wide);
		res = c * textCoverage(maskTex.sample(samp, v.tex).r, unpremul(c), i.inner.x, i.inner.y, i.radii);
	} else if (kind < 3.5f) {
		res = colorTex.sample(samp, v.tex) * i.color.a;
	} else if (kind < 4.5f) {
		res = imageTex.sample(samp, v.tex) * rectCoverage(v.p, i.rect, i.radii);
		if (i.params.y > 0.5f) {
			res.rgb = float3(dot(res.rgb, float3(0.2126f, 0.7152f, 0.0722f)));
		}
	} else {
		float4 c = paint(v.p, i.params.z, i.rect, i.color, i.color2, i.grad, wide);
		float3 straight = unpremul(c);
		float3 a = subpixelCoverage(colorTex.sample(samp, v.tex).rgb, straight, i.inner.x, i.inner.y, i.radii);
		float3 w = a * c.a * rectCoverage(v.p, i.clip, i.clipRadii) * i.params.w;
		float wa = (w.r + w.g + w.b) / 3.0f;
		return PSOut{float4(straight * w, wa), float4(w, wa)};
	}
	float clip = rectCoverage(v.p, i.clip, i.clipRadii);
	res *= clip * i.params.w;
	return PSOut{res, res.aaaa};
}

// The passes computing the backdrop of an effect draw a quad over their
// target, scissored to the texels they compute.
// A pass computing the backdrop of an effect (see package gpu).
struct Pass {
	int2 origin; // down: the first pixel of the area read, in the frame
	int2 limit;  // the last texel that may be read: in the frame for down
	int2 shift;  // down: subtracted from frame pixels to read the source
	int2 dir;    // blur: along rows (1, 0) or columns (0, 1)
	int down;    // down: the size of the squares averaged
	int radius;  // blur: how far it reaches each way
	float sigma; // blur: its standard deviation, 0 for none
	float pad;
};

struct PassVSOut {
	float4 pos [[position]];
};

vertex PassVSOut passVS(uint vid [[vertex_id]]) {
	float2 corner = float2(float(vid & 1u), float(vid >> 1u));
	PassVSOut o;
	o.pos = float4(corner * 2.0f - 1.0f, 0.0f, 1.0f);
	return o;
}

// down averages the square of the area that texel stands for, repeating
// the area's last pixels past its far edges.
fragment float4 down(PassVSOut v [[stage_in]], texture2d<float> src [[texture(0)]], constant Pass &p [[buffer(0)]]) {
	int2 o = p.origin + int2(v.pos.xy) * p.down;
	float4 c = float4(0.0f);
	for (int y = 0; y < p.down; y++) {
		for (int x = 0; x < p.down; x++) {
			c += src.read(uint2(min(o + int2(x, y), p.limit) - p.shift));
		}
	}
	return c * (1.0f / float(p.down * p.down));
}

// blur blurs along a row or a column, clamping at its ends.
fragment float4 blur(PassVSOut v [[stage_in]], texture2d<float> src [[texture(0)]], constant Pass &p [[buffer(0)]]) {
	int2 at = int2(v.pos.xy);
	float4 c = float4(0.0f);
	float sum = 0.0f;
	for (int o = -p.radius; o <= p.radius; o++) {
		float x = float(o);
		float w = p.sigma > 0.0f ? exp(-(x * x) / (2.0f * p.sigma * p.sigma)) : 1.0f;
		c += src.read(uint2(clamp(at + p.dir * o, int2(0), p.limit))) * w;
		sum += w;
	}
	return c / sum;
}

#endif
