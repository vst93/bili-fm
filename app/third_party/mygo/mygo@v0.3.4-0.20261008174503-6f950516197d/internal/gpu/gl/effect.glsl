// The head and the tail of an effect's fragment shader (scene.Effect):
// after shader.glsl with EFFECT defined, the head declares what an effect
// reads, the effect's source follows, then the tail, after the line
// "// effect", draws its instances with it.

uniform sampler2D uBackdrop;

// What an effect reads of its instance.
struct Effect {
	vec4 rect;  // x, y, width, height in pixels
	vec4 radii; // top-left, top-right, bottom-right, bottom-left, circular
	vec4 p0, p1, p2, p3, p4;
	vec4 area;  // where the backdrop's area starts in the frame, and its size in texels
	float down; // the size of the squares the backdrop averages
};

vec3 backdropAt(ivec2 p, ivec2 size) {
	return texelFetch(uBackdrop, clamp(p, ivec2(0), size - 1), 0).rgb;
}

// sampleBackdrop returns the backdrop at q, in the frame's pixels,
// premultiplied, filtered bilinearly from its texels as
// scene.BackdropImage.Sample does.
vec3 sampleBackdrop(Effect e, vec2 q) {
	vec2 u = (q - e.area.xy) / e.down - 0.5;
	vec2 f = floor(u);
	vec2 w = u - f;
	ivec2 p = ivec2(f);
	ivec2 size = ivec2(e.area.zw);
	vec3 top = backdropAt(p, size) * (1.0 - w.x) + backdropAt(p + ivec2(1, 0), size) * w.x;
	vec3 bot = backdropAt(p + ivec2(0, 1), size) * (1.0 - w.x) + backdropAt(p + ivec2(1, 1), size) * w.x;
	return top * (1.0 - w.y) + bot * w.y;
}

// effect

void main() {
	vec2 p = vPoint.xy;
	Effect e = Effect(vRect, abs(vRadii), vInner, vColor, vColor2, vBorder, vGrad, vWidths, vParams.z);
	fragColor = effect(p, e) * (rectCoverage(p, vRect, vRadii) * rectCoverage(p, vClip, vClipRadii) * vParams.w);
#ifdef DUAL
	fragAlpha = fragColor.aaaa;
#endif
}
