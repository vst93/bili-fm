// The one shader of the Direct3D 11 renderer: every scene op is an
// instanced quad, and the pixel shader computes the coverage of rounded
// rectangles, borders, gradients, stripes and shadows from signed
// distances, as the software renderer (internal/raster) does, with the
// passes computing the backdrops of effects (passvs, downps and blurps).
// The pixel shaders of effects (scene.Effect) take its start, with EFFECT
// defined, before effect.hlsl. Colors are straight (not
// premultiplied) and blending happens in sRGB space, as in browsers, with a
// second color for the source's alpha of each channel, which subpixel
// glyphs need.
//
// `go generate` compiles it to DXBC (shaders.go) on Windows.

cbuffer Globals : register(b0) {
	float2 viewport;
	float2 pad;
};

struct Inst {
	float4 rect : RECT;         // x, y, width, height in pixels
	float4 radii : RADII;       // top-left, top-right, bottom-right, bottom-left
	float4 inner : INNER;       // radii of the border's inner edge, or of the box casting a shadow
	float4 color : COLOR0;
	float4 color2 : COLOR1;     // gradient end
	float4 border : COLOR2;     // border color
	float4 grad : GRAD;         // gradient start and end points, or stripes
	float4 uv : UV;             // texture rectangle, normalized, border widths, or the box casting a shadow
	float4 clip : CLIP;         // the innermost clip rectangle
	float4 clipRadii : CLIPR;
	float4 params : PARAMS;     // kind, dashed or grayscale, sigma or paint, opacity
};

struct VSOut {
	float4 pos : SV_Position;
	float2 p : PIXEL;
	float2 tex : TEXCOORD0;
	nointerpolation float4 rect : RECT;
	nointerpolation float4 radii : RADII;
	nointerpolation float4 inner : INNER;
	nointerpolation float4 color : COLOR0;
	nointerpolation float4 color2 : COLOR1;
	nointerpolation float4 border : COLOR2;
	nointerpolation float4 grad : GRAD;
	nointerpolation float4 widths : WIDTHS;
	nointerpolation float4 clip : CLIP;
	nointerpolation float4 clipRadii : CLIPR;
	nointerpolation float4 params : PARAMS;
};

VSOut vs(uint vid : SV_VertexID, Inst i) {
	float2 corner = float2(vid & 1, vid >> 1);
	float4 r = i.rect;
	float kind = i.params.x;
	if (kind < 0.5 || kind > 5.5) { // fills and effects
		r = float4(r.xy - 1, r.zw + 2);
	} else if (kind < 1.5) {
		float e = 3 * i.params.z + 1;
		r = float4(r.xy - e, r.zw + 2 * e);
	}
	float2 p = r.xy + corner * r.zw;
	VSOut o;
	o.pos = float4(p / viewport * float2(2, -2) + float2(-1, 1), 0, 1);
	o.p = p;
	o.tex = lerp(i.uv.xy, i.uv.zw, corner);
	o.rect = i.rect;
	o.radii = i.radii;
	o.inner = i.inner;
	o.color = i.color;
	o.color2 = i.color2;
	o.border = i.border;
	o.grad = i.grad;
	o.widths = i.uv;
	o.clip = i.clip;
	o.clipRadii = i.clipRadii;
	o.params = i.params;
	return o;
}

// The color and, for dual-source blending, the source's alpha of each
// channel.
struct PSOut {
	float4 color : SV_Target0;
	float4 alpha : SV_Target1;
};

Texture2D maskTex : register(t0);
Texture2D colorTex : register(t1);
Texture2D imageTex : register(t2);
SamplerState samp : register(s0);

float sdRoundRect(float2 p, float4 rect, float4 radii) {
	float2 h = rect.zw * 0.5;
	float2 q = p - rect.xy - h;
	float r = q.x < 0 ? (q.y < 0 ? radii.x : radii.w) : (q.y < 0 ? radii.y : radii.z);
	float2 a = abs(q) - h + r;
	return length(max(a, 0)) + min(max(a.x, a.y), 0) - r;
}

float coverage(float d) { return saturate(0.5 - d); }

// rectCoverage returns how much of the pixel at p a rounded rectangle
// covers: by the distance to its edge near rounded corners, and exactly,
// the area of the pixel inside it, near square ones.
float rectCoverage(float2 p, float4 rect, float4 radii) {
	float2 q = p - rect.xy - rect.zw * 0.5;
	float r = q.x < 0 ? (q.y < 0 ? radii.x : radii.w) : (q.y < 0 ? radii.y : radii.z);
	if (r > 0) {
		return coverage(sdRoundRect(p, rect, radii));
	}
	float2 c = saturate(min(rect.xy + rect.zw, p + 0.5) - max(rect.xy, p - 0.5));
	return c.x * c.y;
}

float4 premul(float4 c) { return float4(c.rgb * c.a, c.a); }

float3 unpremul(float4 c) { return c.a > 0 ? c.rgb / c.a : float3(0, 0, 0); }

// What follows is the renderer's own; effects (effect.hlsl) take what is
// above.
#ifndef EFFECT

// textCoverage corrects the coverage a of a glyph of straight color c as
// Direct2D blends text (scene.TextCoverage): it enhances the contrast,
// the more the darker c is, and corrects for gamma with the ratios g.
float textCoverage(float a, float3 c, float contrast, float boost, float4 g) {
	float k = contrast * saturate(3 - 4 * dot(c, float3(0.30, 0.59, 0.11))) + boost;
	a = a * (k + 1) / (a * k + 1);
	float f = dot(c, float3(0.25, 0.5, 0.25));
	return saturate(a + a * (1 - a) * ((g.x * f + g.y) * a + (g.z * f + g.w)));
}

// subpixelCoverage does the same for each subpixel of a subpixel glyph.
float3 subpixelCoverage(float3 a, float3 c, float contrast, float boost, float4 g) {
	float k = contrast * saturate(3 - 4 * dot(c, float3(0.30, 0.59, 0.11))) + boost;
	a = a * (k + 1) / (a * k + 1);
	return saturate(a + a * (1 - a) * ((g.x * c + g.y) * a + (g.z * c + g.w)));
}

float2 erf2(float2 x) {
	float2 s = sign(x), a = abs(x);
	x = 1 + (0.278393 + (0.230389 + 0.078108 * (a * a)) * a) * a;
	x *= x;
	return s - s / (x * x);
}

float gaussian(float x, float sigma) {
	return exp(-(x * x) / (2 * sigma * sigma)) / (2.50662827463 * sigma);
}

// The blurred rounded box of Evan Wallace: exact along x, four samples
// along y.
float shadowX(float x, float y, float sigma, float corner, float2 h) {
	float delta = min(h.y - corner - abs(y), 0);
	float curved = h.x - corner + sqrt(max(0, corner * corner - delta * delta));
	float2 integral = 0.5 + 0.5 * erf2((x + float2(-curved, curved)) * (sqrt(0.5) / sigma));
	return integral.y - integral.x;
}

float boxShadow(float2 p, float4 rect, float sigma, float corner) {
	float2 h = rect.zw * 0.5;
	p -= rect.xy + h;
	float low = p.y - h.y, high = p.y + h.y;
	float start = clamp(-3 * sigma, low, high);
	float end = clamp(3 * sigma, low, high);
	float step = (end - start) / 4;
	float y = start + step * 0.5;
	float v = 0;
	for (int k = 0; k < 4; k++) {
		v += shadowX(p.x, p.y - y, sigma, corner, h) * gaussian(y, sigma) * step;
		y += step;
	}
	return v;
}

float3 toLinear(float3 c) {
	return c <= 0.04045 ? c / 12.92 : pow((c + 0.055) / 1.055, 2.4);
}

float3 toSRGB(float3 c) {
	return c <= 0.0031308 ? c * 12.92 : 1.055 * pow(max(c, 0), 1.0 / 2.4) - 0.055;
}

float3 cbrt3(float3 v) { return sign(v) * pow(abs(v), 1.0 / 3.0); }

float3 oklab(float3 srgb) {
	float3 c = toLinear(srgb);
	float3 lms = cbrt3(float3(
		0.4122214708 * c.r + 0.5363325363 * c.g + 0.0514459929 * c.b,
		0.2119034982 * c.r + 0.6806995451 * c.g + 0.1073969566 * c.b,
		0.0883024619 * c.r + 0.2817188376 * c.g + 0.6299787005 * c.b));
	return float3(
		0.2104542553 * lms.x + 0.7936177850 * lms.y - 0.0040720468 * lms.z,
		1.9779984951 * lms.x - 2.4285922050 * lms.y + 0.4505937099 * lms.z,
		0.0259040371 * lms.x + 0.7827717662 * lms.y - 0.8086757660 * lms.z);
}

float3 fromOklab(float3 lab) {
	float3 lms = float3(
		lab.x + 0.3963377774 * lab.y + 0.2158037573 * lab.z,
		lab.x - 0.1055613458 * lab.y - 0.0638541728 * lab.z,
		lab.x - 0.0894841775 * lab.y - 1.2914855480 * lab.z);
	lms = lms * lms * lms;
	return toSRGB(float3(
		4.0767416621 * lms.x - 3.3077115913 * lms.y + 0.2309699292 * lms.z,
		-1.2684380046 * lms.x + 2.6097574011 * lms.y - 0.3413193965 * lms.z,
		-0.0041960863 * lms.x - 0.7034186147 * lms.y + 1.7076147010 * lms.z));
}

// paint returns the premultiplied color at p of plain color, a gradient
// mixed in sRGB (1) or Oklab (2), or stripes (3), as scene.Paint says.
float4 paint(float2 p, float mode, float4 rect, float4 c1, float4 c2, float4 g) {
	if (mode < 0.5) {
		return premul(c1);
	}
	if (mode < 2.5) {
		float2 d = g.zw - g.xy;
		float t = saturate(dot(p - g.xy, d) / max(dot(d, d), 0.0001));
		float a = lerp(c1.a, c2.a, t);
		if (mode < 1.5) {
			return float4(lerp(c1.rgb * c1.a, c2.rgb * c2.a, t), a);
		}
		float3 lab = lerp(oklab(c1.rgb) * c1.a, oklab(c2.rgb) * c2.a, t);
		return a > 0 ? float4(saturate(fromOklab(lab / a)) * a, a) : float4(0, 0, 0, 0);
	}
	float s = dot(p - rect.xy, g.xy);
	float phase = s - g.w * floor(s / g.w);
	float cov = coverage(min(max(-phase, phase - g.z), g.w - phase));
	return premul(c1) * cov + premul(c2) * (1 - cov);
}

// dash returns how much of a dashed border shows at p: each side, which
// the pixel belongs to when it is nearest that side's edge in widths of
// its border, has an odd number of dashes and gaps of equal length, about
// three widths, starting and ending with a dash.
float dash(float2 p, float4 rect, float4 w) {
	float2 q = p - rect.xy;
	float dt = w.x > 0 ? q.y / w.x : 1e9;
	float dr = w.y > 0 ? (rect.z - q.x) / w.y : 1e9;
	float db = w.z > 0 ? (rect.w - q.y) / w.z : 1e9;
	float dl = w.w > 0 ? q.x / w.w : 1e9;
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
	float n = max(1, floor((len / (3 * bw) + 1) * 0.5 + 0.5));
	float period = 6 * bw;
	n += n * period <= len ? 1 : 0;
	n -= (n - 1) * period > len ? 1 : 0;
	float seg = len / (2 * n - 1);
	float k = floor(s / seg);
	float f = s - k * seg;
	float edge = min(f, seg - f);
	return coverage(k - 2 * floor(k * 0.5) < 0.5 ? -edge : edge);
}

PSOut ps(VSOut i) {
	float kind = i.params.x;
	float4 res;
	if (kind < 0.5) {
		float outer = rectCoverage(i.p, i.rect, i.radii);
		res = paint(i.p, i.params.z, i.rect, i.color, i.color2, i.grad) * outer;
		float4 bw = i.widths; // top, right, bottom, left
		if (any(bw > 0)) {
			float4 ir = float4(i.rect.xy + bw.wx, i.rect.zw - bw.yz - bw.wx);
			float innerCov = (ir.z > 0 && ir.w > 0) ? rectCoverage(i.p, ir, i.inner) : 0;
			float bc = saturate(outer - innerCov);
			if (i.params.y > 0.5) {
				bc *= dash(i.p, i.rect, bw);
			}
			float4 b = premul(i.border) * bc;
			res = b + res * (1 - b.a);
		}
	} else if (kind < 1.5) {
		float sigma = i.params.z;
		float corner = max(max(i.radii.x, i.radii.y), max(i.radii.z, i.radii.w));
		float s = sigma > 0 ? boxShadow(i.p, i.rect, sigma, corner) : rectCoverage(i.p, i.rect, i.radii);
		if (i.widths.z > 0 && i.widths.w > 0) {
			s *= 1 - rectCoverage(i.p, i.widths, i.inner); // outside the box casting it
		}
		res = premul(i.color) * s;
	} else if (kind < 2.5) {
		float4 c = paint(i.p, i.params.z, i.rect, i.color, i.color2, i.grad);
		res = c * textCoverage(maskTex.Sample(samp, i.tex).r, unpremul(c), i.inner.x, i.inner.y, i.radii);
	} else if (kind < 3.5) {
		res = colorTex.Sample(samp, i.tex) * i.color.a;
	} else if (kind < 4.5) {
		res = imageTex.Sample(samp, i.tex) * rectCoverage(i.p, i.rect, i.radii);
		if (i.params.y > 0.5) {
			res.rgb = dot(res.rgb, float3(0.2126, 0.7152, 0.0722));
		}
	} else {
		float4 c = paint(i.p, i.params.z, i.rect, i.color, i.color2, i.grad);
		float3 straight = unpremul(c);
		float3 a = subpixelCoverage(colorTex.Sample(samp, i.tex).rgb, straight, i.inner.x, i.inner.y, i.radii);
		float3 w = a * c.a * rectCoverage(i.p, i.clip, i.clipRadii) * i.params.w;
		float wa = (w.r + w.g + w.b) / 3;
		PSOut so;
		so.color = float4(straight * w, wa);
		so.alpha = float4(w, wa);
		return so;
	}
	float clip = rectCoverage(i.p, i.clip, i.clipRadii);
	PSOut o;
	o.color = res * (clip * i.params.w);
	o.alpha = o.color.aaaa;
	return o;
}

// The passes computing the backdrop of an effect draw a quad over their
// target, scissored to the texels they compute.
cbuffer PassConstants : register(b1) {
	int2 passOrigin; // down: the first pixel of the area read, in the frame
	int2 passLimit;  // the last texel that may be read: in the frame for down
	int2 passShift;  // down: subtracted from frame pixels to read the source
	int2 passDir;    // blur: along rows (1, 0) or columns (0, 1)
	int passDown;    // down: the size of the squares averaged
	int passRadius;  // blur: how far it reaches each way
	float passSigma; // blur: its standard deviation, 0 for none
	float passPad;
};

Texture2D passSrc : register(t4);

float4 passvs(uint vid : SV_VertexID) : SV_Position {
	float2 corner = float2(vid & 1, vid >> 1);
	return float4(corner * 2 - 1, 0, 1);
}

// down averages the square of the area that texel stands for, repeating
// the area's last pixels past its far edges.
float4 downps(float4 pos : SV_Position) : SV_Target {
	int2 o = passOrigin + int2(pos.xy) * passDown;
	float4 c = float4(0, 0, 0, 0);
	for (int y = 0; y < passDown; y++) {
		for (int x = 0; x < passDown; x++) {
			c += passSrc.Load(int3(min(o + int2(x, y), passLimit) - passShift, 0));
		}
	}
	return c * (1.0 / float(passDown * passDown));
}

// blur blurs along a row or a column, clamping at its ends.
float4 blurps(float4 pos : SV_Position) : SV_Target {
	int2 at = int2(pos.xy);
	float4 c = float4(0, 0, 0, 0);
	float sum = 0;
	for (int o = -passRadius; o <= passRadius; o++) {
		float x = float(o);
		float w = passSigma > 0 ? exp(-(x * x) / (2 * passSigma * passSigma)) : 1;
		c += passSrc.Load(int3(clamp(at + passDir * o, int2(0, 0), passLimit), 0)) * w;
		sum += w;
	}
	return c / sum;
}

#endif
