// The head and the tail of an effect's library (scene.Effect): after
// shader.metal with EFFECT defined, the head declares what an effect
// reads, the effect's source follows, then the tail, after the line
// "// effect", draws its instances with it.

// What an effect reads of its instance.
struct Effect {
	float4 rect;  // x, y, width, height in pixels
	float4 radii; // top-left, top-right, bottom-right, bottom-left, circular
	float4 p0, p1, p2, p3, p4;
	float4 area;  // where the backdrop's area starts in the frame, and its size in texels
	float down;   // the size of the squares the backdrop averages
	bool wide;    // the target keeps colors outside the sRGB gamut, in extended sRGB
};

float3 backdropAt(texture2d<float> t, int2 p, int2 size) {
	return t.read(uint2(clamp(p, int2(0), size - 1))).rgb;
}

// sampleBackdrop returns the backdrop at q, in the frame's pixels,
// premultiplied, filtered bilinearly from its texels as
// scene.BackdropImage.Sample does.
float3 sampleBackdrop(texture2d<float> t, Effect e, float2 q) {
	float2 u = (q - e.area.xy) / e.down - 0.5f;
	float2 f = floor(u);
	float2 w = u - f;
	int2 p = int2(f);
	int2 size = int2(e.area.zw);
	float3 top = backdropAt(t, p, size) * (1.0f - w.x) + backdropAt(t, p + int2(1, 0), size) * w.x;
	float3 bot = backdropAt(t, p + int2(0, 1), size) * (1.0f - w.x) + backdropAt(t, p + int2(1, 1), size) * w.x;
	return top * (1.0f - w.y) + bot * w.y;
}

// effect

fragment PSOut effect_ps(VSOut v [[stage_in]],
                         const device Inst *insts [[buffer(0)]],
                         constant float4 &globals [[buffer(1)]],
                         texture2d<float> backdrop [[texture(3)]]) {
	Inst i = insts[v.inst];
	// The shape's corners are continuous where its radii are negative,
	// which the effect takes as circular.
	Effect e = {i.rect, abs(i.radii), i.inner, i.color, i.color2, i.border, i.grad, i.uv, i.params.z, globals.z > 0.5f};
	float4 res = effect(v.p, e, backdrop) * (rectCoverage(v.p, i.rect, i.radii) * rectCoverage(v.p, i.clip, i.clipRadii) * i.params.w);
	return PSOut{res, res.aaaa};
}
