// The head and the tail of an effect's pixel shader (scene.Effect): after
// shader.hlsl with EFFECT defined, the head declares what an effect reads,
// the effect's source follows, then the tail, after the line "// effect",
// draws its instances with it (effectps).

Texture2D backdropTex : register(t3);

// What an effect reads of its instance.
struct Effect {
	float4 rect;  // x, y, width, height in pixels
	float4 radii; // top-left, top-right, bottom-right, bottom-left, circular
	float4 p0, p1, p2, p3, p4;
	float4 area;  // where the backdrop's area starts in the frame, and its size in texels
	float down;   // the size of the squares the backdrop averages
};

float3 backdropAt(int2 p, int2 size) {
	return backdropTex.Load(int3(clamp(p, int2(0, 0), size - 1), 0)).rgb;
}

// sampleBackdrop returns the backdrop at q, in the frame's pixels,
// premultiplied, filtered bilinearly from its texels as
// scene.BackdropImage.Sample does.
float3 sampleBackdrop(Effect e, float2 q) {
	float2 u = (q - e.area.xy) / e.down - 0.5;
	float2 f = floor(u);
	float2 w = u - f;
	int2 p = int2(f);
	int2 size = int2(e.area.zw);
	float3 top = backdropAt(p, size) * (1 - w.x) + backdropAt(p + int2(1, 0), size) * w.x;
	float3 bot = backdropAt(p + int2(0, 1), size) * (1 - w.x) + backdropAt(p + int2(1, 1), size) * w.x;
	return top * (1 - w.y) + bot * w.y;
}

// effect

PSOut effectps(VSOut i) {
	Effect e;
	e.rect = i.rect;
	e.radii = abs(i.radii);
	e.p0 = i.inner;
	e.p1 = i.color;
	e.p2 = i.color2;
	e.p3 = i.border;
	e.p4 = i.grad;
	e.area = i.widths;
	e.down = i.params.z;
	float4 res = effect(i.p, e) * (rectCoverage(i.p, i.rect, i.radii) * rectCoverage(i.p, i.clip, i.clipRadii) * i.params.w);
	PSOut o;
	o.color = res;
	o.alpha = res.aaaa;
	return o;
}
