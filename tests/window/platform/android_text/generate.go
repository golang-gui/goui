//go:build ignore

// Native sources are maintainer inputs; consumer builds use precompiled components.
package main

import "github.com/golang-gui/goui/internal/nativegen"

func main() {
	nativegen.Main("tests/window/platform/android_text", map[string]string{
		"native/BackendReference.java": `package org.golanggui.androidtext;

import android.graphics.Bitmap;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.graphics.Typeface;
import android.icu.text.BreakIterator;
import android.text.Layout;
import android.text.StaticLayout;
import android.text.TextPaint;
import org.json.JSONArray;
import org.json.JSONObject;
import java.io.File;
import java.io.FileOutputStream;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.Locale;

// 独立原生参考：不调用生产 helper，直接创建同字体/单位的公开 StaticLayout。
final class BackendReference {
    static void write(String directory) throws Exception {
        File dir=new File(directory);JSONArray cases=new JSONArray();
        String[] texts={"office a\u0301", "ABC \u05d0\u05d1\u05d2 DEF", "\u0627\u0644\u0639\u0631\u0628\u064a\u0629", "A\ud83d\ude00\ud83d\udc69\u200d\ud83d\udcbb"};
        for(int k=0;k<texts.length;k++){
            String text=texts[k];TextPaint paint=new TextPaint(Paint.ANTI_ALIAS_FLAG|Paint.SUBPIXEL_TEXT_FLAG|Paint.LINEAR_TEXT_FLAG);
            paint.setTypeface(Typeface.createFromFile(new File(dir,"regular.ttf")));paint.setTextSize(24*64);paint.setColor(0xff000000);
            StaticLayout layout=StaticLayout.Builder.obtain(text,0,text.length(),paint,220*64).setIncludePad(false)
                .setUseLineSpacingFromFallbacks(true).setBreakStrategy(Layout.BREAK_STRATEGY_SIMPLE)
                .setHyphenationFrequency(Layout.HYPHENATION_FREQUENCY_NONE).build();
            JSONArray lines=new JSONArray(),carets=new JSONArray(),rasters=new JSONArray();
            for(int i=0;i<layout.getLineCount();i++)lines.put(new JSONObject().put("y",layout.getLineTop(i)/64f)
                .put("height",(layout.getLineBottom(i)-layout.getLineTop(i))/64f).put("baseline",layout.getLineBaseline(i)/64f)
                .put("width",layout.getLineWidth(i)/64f));
            BreakIterator iterator=BreakIterator.getCharacterInstance(Locale.ROOT);iterator.setText(text);
            for(int offset=iterator.first();offset!=BreakIterator.DONE;offset=iterator.next())
                carets.put(new JSONObject().put("offset",text.substring(0,offset).getBytes(StandardCharsets.UTF_8).length)
                    .put("x",layout.getPrimaryHorizontal(offset)/64f).put("line",layout.getLineForOffset(offset)));
            for(float scale:new float[]{1,1.5f,2}){
                Bitmap bitmap=Bitmap.createBitmap(384,192,Bitmap.Config.ARGB_8888);bitmap.setDensity(Bitmap.DENSITY_NONE);
                Canvas canvas=new Canvas(bitmap);canvas.scale(scale/64,scale/64);layout.draw(canvas);
                ByteBuffer bytes=ByteBuffer.allocate(bitmap.getByteCount());bitmap.copyPixelsToBuffer(bytes);
                String name="reference-"+k+"-"+scale+".rgba";
                try(FileOutputStream out=new FileOutputStream(new File(dir,name))){out.write(bytes.array());}
                rasters.put(new JSONObject().put("scale",scale).put("file",name));bitmap.recycle();
            }
            cases.put(new JSONObject().put("text",text).put("lines",lines).put("carets",carets).put("rasters",rasters));
        }
        // 两端对齐的独立参考直接使用 Canvas.drawText + Paint.wordSpacing，
        // 不读取生产 helper 的 Cluster，也不使用已发现不自洽的原生 justification。
        String fill="one two three four five six";
        TextPaint p=new TextPaint(Paint.ANTI_ALIAS_FLAG|Paint.SUBPIXEL_TEXT_FLAG|Paint.LINEAR_TEXT_FLAG);
        p.setTypeface(Typeface.createFromFile(new File(dir,"regular.ttf")));p.setTextSize(24*64);p.setColor(0xff000000);
        StaticLayout base=StaticLayout.Builder.obtain(fill,0,fill.length(),p,160*64).setIncludePad(false)
            .setUseLineSpacingFromFallbacks(true).setBreakStrategy(Layout.BREAK_STRATEGY_SIMPLE).setHyphenationFrequency(Layout.HYPHENATION_FREQUENCY_NONE).build();
        int end=base.getLineVisibleEnd(0),spaces=0;for(int i=0;i<end;i++)if(fill.charAt(i)==' ')spaces++;
        p.setWordSpacing((float)Math.floor((160*64-p.measureText(fill,0,end))/spaces*64)/64);
        Bitmap justified=Bitmap.createBitmap(240,42,Bitmap.Config.ARGB_8888);Canvas c=new Canvas(justified);c.scale(1.5f/64,1.5f/64);
        c.drawText(fill,0,end,0,base.getLineBaseline(0),p);ByteBuffer buffer=ByteBuffer.allocate(justified.getByteCount());justified.copyPixelsToBuffer(buffer);
        try(FileOutputStream out=new FileOutputStream(new File(dir,"reference-fill.rgba"))){out.write(buffer.array());}justified.recycle();
        String decorated="M   M";p.setWordSpacing(0);p.setColor(0x80ff0000);
        StaticLayout decorationBase=StaticLayout.Builder.obtain(decorated,0,decorated.length(),p,220*64).setIncludePad(false).build();
        for(String kind:new String[]{"underline","strike"}){
            p.setUnderlineText(kind.equals("underline"));p.setStrikeThruText(kind.equals("strike"));
            Bitmap bitmap=Bitmap.createBitmap(330,96,Bitmap.Config.ARGB_8888);Canvas target=new Canvas(bitmap);target.scale(1.5f/64,1.5f/64);
            target.drawText(decorated,0,decorationBase.getLineBaseline(0),p);ByteBuffer pixels=ByteBuffer.allocate(bitmap.getByteCount());bitmap.copyPixelsToBuffer(pixels);
            try(FileOutputStream out=new FileOutputStream(new File(dir,"reference-"+kind+".rgba"))){out.write(pixels.array());}bitmap.recycle();
        }
        try(FileOutputStream out=new FileOutputStream(new File(dir,"backend-reference.json"))){out.write(cases.toString(2).getBytes(StandardCharsets.UTF_8));}
    }
}
`,
		"native/LayoutInteractionProbe.java": `package org.golanggui.androidtext;

import android.graphics.Paint;
import android.graphics.Path;
import android.graphics.Rect;
import android.graphics.RectF;
import android.graphics.Region;
import android.graphics.RegionIterator;
import android.graphics.Typeface;
import android.icu.text.BreakIterator;
import android.text.Layout;
import android.text.SpannableString;
import android.text.Spanned;
import android.text.StaticLayout;
import android.text.TextPaint;
import android.text.style.ForegroundColorSpan;
import android.text.style.RelativeSizeSpan;
import android.text.style.TabStopSpan;
import org.json.JSONArray;
import org.json.JSONObject;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Locale;

// 能力实验：公开 ICU 边界 + 同一个 Layout 的几何，不依赖私有 shaping 库。
final class LayoutInteractionProbe {
    private final Typeface font;
    private final JSONArray checks = new JSONArray();
    private final JSONArray cases = new JSONArray();
    private int failures;

    private LayoutInteractionProbe(Typeface font) { this.font = font; }

    private void check(String name, boolean pass, Object detail) throws Exception {
        checks.put(new JSONObject().put("name", name).put("pass", pass).put("detail", detail));
        if (!pass) failures++;
    }

    private static int utf8(String text, int offset) {
        return text.substring(0, offset).getBytes(StandardCharsets.UTF_8).length;
    }

    private static JSONObject rect(RectF r) throws Exception {
        return new JSONObject().put("x", r.left).put("y", r.top)
            .put("width", r.width()).put("height", r.height());
    }

    private static ArrayList<Integer> boundaries(String text) {
        BreakIterator iterator = BreakIterator.getCharacterInstance(Locale.ROOT);
        iterator.setText(text);
        ArrayList<Integer> result = new ArrayList<>();
        for (int i = iterator.first(); i != BreakIterator.DONE; i = iterator.next()) result.add(i);
        return result;
    }

    // GOUI 选区覆盖文本逻辑范围，不扩展到软换行后的空白窗口区域。
    private static RectF clip(StaticLayout l, int line) {
        float width = l.getLineWidth(line);
        float x = l.getParagraphDirection(line) == Layout.DIR_RIGHT_TO_LEFT
            ? l.getLineRight(line) - width : l.getLineLeft(line);
        int end = l.getLineEnd(line);
        CharSequence text = l.getText();
        if (end > l.getLineStart(line) && text.charAt(end-1) == '\n') {
            // getLineWidth 会计入换行符的布局宽度；GOUI 不绘制该分隔符选区。
            end--;
            if (end > l.getLineStart(line) && text.charAt(end-1) == '\r') end--;
            float edge = l.getPrimaryHorizontal(end);
            if (l.getParagraphDirection(line) == Layout.DIR_RIGHT_TO_LEFT) {
                x = edge; width = l.getLineRight(line) - edge;
            } else width = edge - x;
        }
        return new RectF(x, l.getLineTop(line), x + width, l.getLineBottom(line));
    }

    private static Path selected(StaticLayout l, int start, int end, int line) {
        Path p = new Path(); l.getSelectionPath(start, end, p);
        Path limit = new Path(); limit.addRect(clip(l, line), Path.Direction.CW);
        p.op(limit, Path.Op.INTERSECT);
        return p;
    }

    private static JSONArray regions(StaticLayout l, int start, int end) throws Exception {
        JSONArray rects = new JSONArray();
        // RegionIterator 保留 bidi 不连续选区，不能用整体 bounding box 充当参考。
        for (int line = 0; line < l.getLineCount(); line++) {
            Path p = selected(l, start, end, line);
            Region region = new Region();
            region.setPath(p, new Region(-4096, -4096, 4096, 4096));
            RegionIterator iterator = new RegionIterator(region); Rect r = new Rect();
            while (iterator.next(r)) rects.put(rect(new RectF(r)));
        }
        return rects;
    }

    private static JSONObject hit(StaticLayout l, ArrayList<Integer> stops, String text, int line, float x, float y) throws Exception {
        int raw = l.getOffsetForHorizontal(line, x), expected = raw;
        float rawPrimary = l.getPrimaryHorizontal(raw), rawSecondary = l.getSecondaryHorizontal(raw);
        float expectedX = rawPrimary;
        String reason = "native";
        if (!stops.contains(raw)) {
            // 原生字体 span 可在字素内部创建停靠点；GOUI 契约只允许字素两端。
            int before = 0, after = text.length();
            for (int s : stops) { if (s < raw) before = s; if (s > raw) { after = s; break; } }
            float a = l.getPrimaryHorizontal(before), b = l.getPrimaryHorizontal(after);
            expected = Math.abs(x-a) < Math.abs(x-b) ? before : after;
            expectedX = expected == before ? a : b;
            reason = "grapheme-snap";
        }
        int end = l.getLineEnd(line);
        if (line+1 < l.getLineCount() && end > l.getLineStart(line) && text.charAt(end-1) != '\n') {
            int previous = l.getLineStart(line);
            for (int s : stops) { if (s >= end) break; if (s > previous) previous = s; }
            Path last = selected(l, previous, end, line); RectF bounds = new RectF(); last.computeBounds(bounds, true);
            float edge = l.isRtlCharAt(previous) ? bounds.left : bounds.right;
            // Android 11 Layout.java 显式限制非末行 max；GOUI 可保留行尾上游亲和性。
            if (raw == previous && Math.abs(x-edge) < Math.abs(x-rawPrimary)) {
                expected = end; expectedX = edge; reason = "wrap-upstream";
            }
        }
        return new JSONObject().put("x", x).put("y", y).put("line", line)
            .put("offset", utf8(text, raw)).put("primary", rawPrimary).put("secondary", rawSecondary)
            .put("nativeLine", l.getLineForOffset(raw)).put("expectedOffset", utf8(text, expected))
            .put("expectedX", expectedX).put("reason", reason);
    }

    private void add(String name, CharSequence source, int width, int... expected) throws Exception {
        String text = source.toString();
        TextPaint paint = new TextPaint(Paint.ANTI_ALIAS_FLAG | Paint.SUBPIXEL_TEXT_FLAG | Paint.LINEAR_TEXT_FLAG);
        paint.setTypeface(font); paint.setTextSize(40); paint.setColor(0xff204060);
        paint.setFontFeatureSettings("'liga' 1, 'clig' 1");
        StaticLayout l = StaticLayout.Builder.obtain(source, 0, source.length(), paint, width)
            .setIncludePad(false).setBreakStrategy(Layout.BREAK_STRATEGY_SIMPLE)
            .setHyphenationFrequency(Layout.HYPHENATION_FREQUENCY_NONE).build();
        ArrayList<Integer> stops = boundaries(text);
        JSONArray utf8Stops = new JSONArray();
        for (int s : stops) utf8Stops.put(utf8(text, s));
        if (expected.length > 0) {
            JSONArray wanted = new JSONArray(); for (int s : expected) wanted.put(s);
            check(name + ":atomic-boundaries", wanted.toString().equals(utf8Stops.toString()), utf8Stops);
        }
        JSONArray lines = new JSONArray(), units = new JSONArray(), carets = new JSONArray(), hits = new JSONArray(), selections = new JSONArray();
        for (int line = 0; line < l.getLineCount(); line++) {
            int start = l.getLineStart(line), end = l.getLineEnd(line);
            RectF bounds = clip(l, line);
            lines.put(rect(bounds).put("start", utf8(text, start)).put("length", utf8(text, end)-utf8(text, start))
                .put("baseline", l.getLineBaseline(line)));
            check(name + ":line-boundaries:" + line, stops.contains(start) && stops.contains(end), start + ":" + end);
            for (float x = bounds.left - 2; x <= bounds.right + 2; x += 2.75f) {
                hits.put(hit(l, stops, text, line, x, bounds.centerY()));
            }
        }
        for (int i = 0; i < stops.size(); i++) {
            int start = stops.get(i);
            carets.put(new JSONObject().put("offset", utf8(text, start)).put("line", l.getLineForOffset(start))
                .put("primary", l.getPrimaryHorizontal(start)).put("secondary", l.getSecondaryHorizontal(start)));
            if (i+1 == stops.size()) continue;
            int end = stops.get(i+1), line = l.getLineForOffset(start);
            if (text.substring(start, end).equals("\n") || text.substring(start, end).equals("\r\n")) continue;
            Path path = selected(l, start, end, line); RectF r = new RectF(); path.computeBounds(r, true);
            if (path.isEmpty() || r.width() == 0) {
                float x = l.getPrimaryHorizontal(start);
                r.set(x, l.getLineTop(line), x, l.getLineBottom(line));
            }
            units.put(rect(r).put("start", utf8(text, start)).put("length", utf8(text, end)-utf8(text, start))
                .put("lineIndex", line).put("direction", l.isRtlCharAt(start) ? 1 : 0));
        }
        // 所有合法范围均直接向 Layout 请求原生选区，和按单元合成的 Go 结果独立比较。
        for (int i = 0; i < stops.size(); i++) for (int j = i+1; j < stops.size(); j++) {
            int start = stops.get(i), end = stops.get(j);
            selections.put(new JSONObject().put("start", utf8(text, start)).put("end", utf8(text, end))
                .put("rects", regions(l, start, end)));
        }
        cases.put(new JSONObject().put("name", name).put("text", text).put("boundaries", utf8Stops)
            .put("lines", lines).put("clusters", units).put("carets", carets).put("hits", hits).put("selections", selections));
    }

    static JSONObject run(Typeface font) throws Exception {
        LayoutInteractionProbe p = new LayoutInteractionProbe(font);
        p.add("ligature", "ffi office", 600, 0,1,2,3,4,5,6,7,8,9,10);
        p.add("combining", "a\u0301b", 600, 0,3,4);
        p.add("emoji", "A\ud83d\ude00\ud83d\udc69\u200d\ud83d\udcbb\ud83c\udde8\ud83c\uddf3B", 600, 0,1,5,16,24,25);
        p.add("indic-vowel", "\u0915\u093f", 600, 0,6);
        p.add("indic-conjunct", "\u0915\u094d\u0937", 600);
        p.add("arabic", "\u0644\u0627 \u0633\u0644\u0627\u0645", 600);
        p.add("bidi", "ABC \u05d0\u05d1\u05d2 DEF", 600);
        p.add("zero-width", "a\u200b\u200bB", 600, 0,1,4,7,8);
        p.add("zero-only", "\u200b", 600, 0,3);
        p.add("rtl-zero", "\u0644\u200b\u0627", 600, 0,2,5,7);
        p.add("wrap", "alpha beta gamma", 105);
        p.add("bidi-wrap", "AB \u05d0\u05d1\u05d2 CD \u05d3\u05d4\u05d5 EF", 150);
        p.add("empty", "", 200, 0);
        p.add("newlines", "A\nB\n", 200, 0,1,2,3,4);
        p.add("rtl-newlines", "\u05d0\u05d1\u05d2\n\u05d3\u05d4\u05d5\n", 200);
        SpannableString color = new SpannableString("a\u0301b");
        color.setSpan(new ForegroundColorSpan(0xffff0000), 1,2, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        p.add("color-in-grapheme", color, 600, 0,3,4);
        SpannableString size = new SpannableString("a\u0301b");
        size.setSpan(new RelativeSizeSpan(1.25f), 1,2, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        p.add("font-in-grapheme", size, 600, 0,3,4);
        SpannableString tabs = new SpannableString("\tX\t\tY");
        for (int stop : new int[]{80,160,240}) tabs.setSpan(new TabStopSpan.Standard(stop),0,tabs.length(),Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        p.add("tabs", tabs, 600, 0,1,2,3,4,5);
        return new JSONObject().put("checks", p.checks).put("failures", p.failures).put("cases", p.cases)
            .put("segmentation", "android.icu.text.BreakIterator.getCharacterInstance(Locale.ROOT)");
    }
}
`,
		"native/TextProbe.java": `package org.golanggui.androidtext;

import android.content.Context;
import android.graphics.Bitmap;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.graphics.Rect;
import android.graphics.Typeface;
import android.os.Build;
import android.os.Looper;
import android.text.Layout;
import android.text.SpannableString;
import android.text.Spanned;
import android.text.StaticLayout;
import android.text.TextPaint;
import android.text.style.ForegroundColorSpan;
import android.text.style.ReplacementSpan;
import android.text.style.TabStopSpan;
import org.json.JSONArray;
import org.json.JSONObject;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;

// 仅用于公开系统 API 的能力验证，不是生产 TextLayout 实现。
public final class TextProbe {
    private final JSONArray checks = new JSONArray();
    private int failures;
    private int candidatesFailed;
    private final Typeface fixture;

    private TextProbe(Context context) { fixture = Typeface.createFromAsset(context.getAssets(), "shape.ttf"); }

    private void check(String name, boolean pass, Object detail) throws Exception {
        checks.put(new JSONObject().put("name", name).put("pass", pass).put("detail", detail));
        if (!pass) failures++;
    }

    // 保留早期 shaping 等价实验；差异不等于交互单元契约失败。
    private void candidate(String name, boolean pass, Object detail) throws Exception {
        checks.put(new JSONObject().put("name", name).put("pass", pass).put("candidate", true).put("detail", detail));
        if (!pass) candidatesFailed++;
    }

    private TextPaint paint() {
        // 关闭 grid fitting，验证小数逻辑度量；记录其对原生 glyph cache 的取舍。
        TextPaint p = new TextPaint(Paint.ANTI_ALIAS_FLAG | Paint.SUBPIXEL_TEXT_FLAG | Paint.LINEAR_TEXT_FLAG);
        p.setTypeface(fixture);
        p.setTextSize(40);
        p.setColor(0xff204060);
        p.setFontFeatureSettings("'liga' 1, 'clig' 1");
        return p;
    }

    private static StaticLayout layout(CharSequence text, TextPaint paint, int width) {
        return StaticLayout.Builder.obtain(text, 0, text.length(), paint, width)
            .setIncludePad(false).setBreakStrategy(Layout.BREAK_STRATEGY_SIMPLE)
            .setHyphenationFrequency(Layout.HYPHENATION_FREQUENCY_NONE).build();
    }

    private static Bitmap raster(StaticLayout l, float scale, Rect region) {
        Bitmap b = Bitmap.createBitmap(region.width(), region.height(), Bitmap.Config.ARGB_8888);
        Canvas c = new Canvas(b);
        c.translate(-region.left, -region.top);
        c.scale(scale, scale);
        l.draw(c);
        return b;
    }

    private static byte[] bytes(Bitmap b) {
        ByteBuffer buffer = ByteBuffer.allocate(b.getByteCount());
        b.copyPixelsToBuffer(buffer);
        return buffer.array();
    }

    private static int utf8Offset(String s, int utf16) {
        return s.substring(0, utf16).getBytes(StandardCharsets.UTF_8).length;
    }

    private static String read(InputStream in) throws Exception {
        try (InputStream source = in; java.io.ByteArrayOutputStream out = new java.io.ByteArrayOutputStream()) {
            byte[] buffer = new byte[4096];
            for (int n; (n = source.read(buffer)) >= 0;) out.write(buffer, 0, n);
            return new String(out.toByteArray(), StandardCharsets.UTF_8);
        }
    }

    private void clusters(Context context, JSONObject report) throws Exception {
        JSONObject reference = new JSONObject(read(context.getAssets().open("shape.json")));
        JSONArray observations = new JSONArray();
        JSONArray cases = reference.getJSONArray("cases");
        int ligatures = 0;
        for (int k = 0; k < cases.length(); k++) {
            JSONObject ref = cases.getJSONObject(k);
            String s = ref.getString("text");
            boolean rtl = ref.getBoolean("rtl");
            TextPaint p = paint();
            JSONArray boundaries = new JSONArray(), zeroAdvances = new JSONArray();
            for (int i = 0; i <= s.length(); i++) {
                if (p.getTextRunCursor(s, 0, s.length(), rtl, i, Paint.CURSOR_AT) == i)
                    boundaries.put(utf8Offset(s, i));
            }
            float[] advances = new float[s.length()];
            p.getTextRunAdvances(s.toCharArray(), 0, s.length(), 0, s.length(), rtl, advances, 0);
            for (int i = 0; i < s.length(); i++) if (advances[i] != 0) zeroAdvances.put(utf8Offset(s, i));
            zeroAdvances.put(utf8Offset(s, s.length()));
            JSONArray wanted = ref.getJSONArray("boundaries");
            StaticLayout l = layout(s, p, 600);
            JSONArray caretX = new JSONArray(), advanceJSON = new JSONArray();
            for (int i = 0; i <= s.length(); i++) caretX.put(l.getPrimaryHorizontal(i));
            for (float x : advances) advanceJSON.put(x);
            JSONObject observation = new JSONObject().put("text", s).put("reference", ref)
                .put("cursorBoundariesUtf8", boundaries).put("nonzeroAdvancesUtf8", zeroAdvances)
                .put("advances", advanceJSON).put("primaryHorizontal", caretX);
            observations.put(observation);
            candidate("cursor-is-shaping-cluster:" + k, wanted.toString().equals(boundaries.toString()), observation);
            candidate("zero-advance-is-shaping-cluster:" + k, wanted.toString().equals(zeroAdvances.toString()), observation);
            if (s.equals("ffi") && ref.getJSONArray("glyphs").length() == 1) {
                ligatures++;
                // 固定字体 cmap 中 U+FB03 与参考 ffi 使用同一个 glyph，独立确认
                // Android 实际也绘制了连字，而不是仅凭另一套引擎的结果推断。
                Bitmap shaped = raster(l,1,new Rect(0,0,200,80));
                Bitmap direct = raster(layout("\ufb03",paint(),600),1,new Rect(0,0,200,80));
                check("native-ffi-presentation-glyph",Arrays.equals(bytes(shaped),bytes(direct)),"ffi equals U+FB03 pixels");
                shaped.recycle();direct.recycle();
            }
        }
        check("independent-fixture-has-ffi-ligature", ligatures == 1, reference.getString("fontSha256"));
        report.put("shapingReference", reference).put("clusterObservations", observations);

        String[] text = {"\ud83d\ude00", "a\u0301", "\ud83d\udc69\u200d\ud83d\udcbb", "\ud83c\udde8\ud83c\uddf3", "\u0915\u093f"};
        for (int k = 0; k < text.length; k++) {
            String s = text[k]; TextPaint p = paint(); int interior = 0;
            for (int i = 1; i < s.length(); i++)
                if (p.getTextRunCursor(s, 0, s.length(), false, i, Paint.CURSOR_AT) == i) interior++;
            check("unicode-caret-interior:" + k, interior == 0, new JSONObject().put("text", s).put("interior", interior));
        }
    }

    private void paragraphs(JSONObject report) throws Exception {
        TextPaint p = paint();
        StaticLayout l = layout("A\nB\n", p, 400);
        check("trailing-newline", l.getLineCount() == 3 && l.getLineStart(2) == 4, l.getLineCount());
        check("empty-layout", layout("", paint(), 100).getLineCount() == 1, "native empty line retained");
        StaticLayout wrapped = layout("alpha beta gamma delta epsilon", paint(), 100);
        check("word-wrap", wrapped.getLineCount() > 2 && wrapped.getLineEnd(wrapped.getLineCount()-1) == wrapped.getText().length(), wrapped.getLineCount());
        StaticLayout bidi = layout("ABC \u05d0\u05d1\u05d2 DEF", paint(), 600);
        check("bidi-run", bidi.isRtlCharAt(4) && bidi.isRtlCharAt(5) && bidi.isRtlCharAt(6)
            && bidi.getPrimaryHorizontal(5) > bidi.getPrimaryHorizontal(6), "Hebrew internal caret order");
        JSONArray lines = new JSONArray();
        for (int i = 0; i < wrapped.getLineCount(); i++) {
            int start = wrapped.getLineStart(i), end = wrapped.getLineEnd(i);
            check("line-range:"+i, start >= 0 && end >= start && wrapped.getLineBaseline(i) > wrapped.getLineTop(i)
                && (i == 0 || start == wrapped.getLineEnd(i-1)), start + ":" + end);
            lines.put(new JSONObject().put("start", start).put("end", end).put("baseline", wrapped.getLineBaseline(i)).put("width", wrapped.getLineWidth(i)));
        }
        report.put("wrappedLines", lines);
        TextPaint fractional = paint(); fractional.setTextSize(19.25f);
        StaticLayout fractions = layout("WWW", fractional, 500);
        check("fractional-advance", Math.abs(fractions.getLineWidth(0)-Math.round(fractions.getLineWidth(0))) > 0.001, fractions.getLineWidth(0));
        TextPaint hinted = paint(); hinted.setLinearText(false); hinted.setTextSize(19.25f);
        report.put("fractionalMeasurement", new JSONObject().put("linear", fractions.getLineWidth(0))
            .put("hinted",layout("WWW",hinted,500).getLineWidth(0)).put("linearTextFlag",true));
        report.put("layoutWidthType", "StaticLayout.Builder width is integer; fractional logical constraints need an explicit mapping");
    }

    private static final class Box extends ReplacementSpan {
        @Override public int getSize(Paint p, CharSequence text, int start, int end, Paint.FontMetricsInt fm) {
            if (fm != null) { fm.top = fm.ascent = -28; fm.descent = fm.bottom = 14; }
            return 37;
        }
        @Override public void draw(Canvas c, CharSequence text, int start, int end, float x, int top, int y, int bottom, Paint p) {}
    }

    private void spans() throws Exception {
        SpannableString text = new SpannableString("\tX\t\tY");
        text.setSpan(new TabStopSpan.Standard(80), 0, text.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        // TabStopSpan 给出绝对停靠点；重复间隔需要显式提供后续 stop。
        text.setSpan(new TabStopSpan.Standard(160), 0, text.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        text.setSpan(new TabStopSpan.Standard(240), 0, text.length(), Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        StaticLayout tabs = layout(text, paint(), 600);
        check("tab-stops", tabs.getPrimaryHorizontal(1) == 80 && tabs.getPrimaryHorizontal(3) == 160 && tabs.getPrimaryHorizontal(4) == 240,
            Arrays.toString(new float[]{tabs.getPrimaryHorizontal(1), tabs.getPrimaryHorizontal(3), tabs.getPrimaryHorizontal(4)}));
        SpannableString boxes = new SpannableString("\ufffc\ufffc");
        boxes.setSpan(new Box(), 0, 1, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        boxes.setSpan(new Box(), 1, 2, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        StaticLayout placeholder = layout(boxes, paint(), 400);
        check("adjacent-placeholders", placeholder.getPrimaryHorizontal(1) == 37 && placeholder.getPrimaryHorizontal(2) == 74 && placeholder.getHeight() >= 42, placeholder.getHeight());
        Bitmap blank = raster(placeholder, 1, new Rect(0, 0, 100, 60));
        boolean empty = true; for (byte x : bytes(blank)) empty &= x == 0;
        check("placeholder-no-glyph", empty, "no replacement tofu painted"); blank.recycle();

        StaticLayout peer = layout("ABCD", paint(), 300);
        Bitmap before = raster(peer, 1, new Rect(0, 0, 300, 80));
        SpannableString colored = new SpannableString("ABCD");
        colored.setSpan(new ForegroundColorSpan(0x80ff0000), 1, 3, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE);
        StaticLayout other = layout(colored, paint(), 300);
        Bitmap changed = raster(other, 1, new Rect(0, 0, 300, 80));
        Bitmap after = raster(peer, 1, new Rect(0, 0, 300, 80));
        check("layout-isolation", Arrays.equals(bytes(before), bytes(after)) && !Arrays.equals(bytes(before), bytes(changed)), "distinct TextPaint/Spannable/Layout");
        check("color-keeps-metrics", peer.getLineWidth(0) == other.getLineWidth(0), peer.getLineWidth(0));
        before.recycle();changed.recycle();after.recycle();
    }

    private void rasterRegions() throws Exception {
        StaticLayout l = layout("office a\u0301\nABC \u05d0\u05d1\u05d2", paint(), 260);
        float width = l.getLineWidth(0); int baseline = l.getLineBaseline(0);
        for (int scale : new int[]{1,2}) {
            Bitmap full = raster(l, scale, new Rect(0, 0, 600, 300));
            Rect[] regions = {new Rect(10, 5, 180, 70), new Rect(-7,-11,210,90), new Rect(610,0,650,60)};
            for (int k = 0; k < regions.length; k++) {
                Rect r = regions[k]; Bitmap part = raster(l, scale, r); boolean equal = true;
                for (int y = 0; y < r.height(); y++) for (int x = 0; x < r.width(); x++) {
                    int sx = r.left+x, sy = r.top+y;
                    int want = sx>=0 && sy>=0 && sx<full.getWidth() && sy<full.getHeight() ? full.getPixel(sx,sy) : 0;
                    equal &= part.getPixel(x,y) == want;
                }
                check("raster-region:"+scale+":"+k, equal, r.toString()); part.recycle();
            }
            check("raster-keeps-layout:"+scale, l.getLineWidth(0) == width && l.getLineBaseline(0) == baseline, width);
            full.recycle();
        }
        Bitmap alpha = Bitmap.createBitmap(16,16,Bitmap.Config.ARGB_8888);Canvas c = new Canvas(alpha);
        Paint p = new Paint();p.setColor(0x80402010);c.drawRect(0,0,16,16,p);
        byte[] pixel = bytes(alpha);
        check("raw-premultiplied-rgba", (pixel[0]&255)==32 && (pixel[1]&255)==16 && (pixel[2]&255)==8 && (pixel[3]&255)==128,
            Arrays.toString(Arrays.copyOf(pixel,4)));alpha.recycle();
    }

    private Bitmap picture() {
        Bitmap b = Bitmap.createBitmap(800,1100,Bitmap.Config.ARGB_8888);Canvas c = new Canvas(b);c.drawColor(0xffeef4fa);
        String[] rows = {"Android system text / JNI", "Latin: office affinity ffi", "\u4e2d\u6587\uff1a\u7a97\u53e3\u5185\u7684 Go", "\u65e5\u672c\u8a9e\uff1a\u304b\u306a\u3068\u6f22\u5b57", "\ud55c\uad6d\uc5b4: \uc548\ub155\ud558\uc138\uc694", "\u0627\u0644\u0639\u0631\u0628\u064a\u0629: \u0627\u0644\u0633\u0644\u0627\u0645 \u0639\u0644\u064a\u0643\u0645", "\u05e2\u05d1\u05e8\u05d9\u05ea: ABC \u05d0\u05d1\u05d2 DEF", "\u0939\u093f\u0928\u094d\u0926\u0940: \u0928\u092e\u0938\u094d\u0924\u0947", "Emoji: \ud83d\ude00 \ud83d\udc69\u200d\ud83d\udcbb \ud83c\udde8\ud83c\uddf3", "Core failures: " + failures, "Shaping boundary differences: " + candidatesFailed};
        int y = 26;
        for (String row : rows) {
            TextPaint p = paint(); p.setTypeface(Typeface.DEFAULT); p.setTextSize(32);
            StaticLayout l = layout(row,p,748);c.save();c.translate(26,y);l.draw(c);c.restore();y+=l.getHeight()+32;
        }
        return b;
    }

    public static byte[] run(Context context, String directory) throws Exception {
        TextProbe p = new TextProbe(context);
        JSONObject report = new JSONObject().put("api",Build.VERSION.SDK_INT).put("release",Build.VERSION.RELEASE)
            .put("thread",Thread.currentThread().getName()).put("workerThread",Looper.myLooper()!=Looper.getMainLooper());
        p.check("jni-worker-thread",Looper.myLooper()!=Looper.getMainLooper(), Thread.currentThread().getName());
        p.clusters(context, report);p.paragraphs(report);p.spans();p.rasterRegions();
        report.put("interaction", LayoutInteractionProbe.run(p.fixture));
        report.put("checks",p.checks).put("coreFailures",p.failures).put("candidateFailures",p.candidatesFailed)
            .put("candidateCompatible",p.candidatesFailed==0).put("width",800).put("height",1100);
        File dir = new File(directory);
        try (FileOutputStream out = new FileOutputStream(new File(dir,"report.json"))) { out.write(report.toString(2).getBytes(StandardCharsets.UTF_8)); }
        Bitmap b = p.picture();
        try (FileOutputStream out = new FileOutputStream(new File(dir,"text.png"))) { b.compress(Bitmap.CompressFormat.PNG,100,out); }
        byte[] pixels = bytes(b);b.recycle();return pixels;
    }

    public static void reference(String directory) throws Exception { BackendReference.write(directory); }
}
`,
		"native/android.c": `// 测试包装：复用正式宿主，仅增加从 Go 线程调用 Java 能力测试的固定入口。
#include <android/native_activity.h>
#include <android/bitmap.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

// 用 NDK 头文件独立校验 Go JNI 绑定使用的表位置与结构体布局。
#define JNI_SLOT(name,index) _Static_assert(offsetof(struct JNINativeInterface,name)==(index)*sizeof(void*),"JNI slot " #name)
JNI_SLOT(FindClass,6); JNI_SLOT(ExceptionOccurred,15); JNI_SLOT(ExceptionClear,17);
JNI_SLOT(PushLocalFrame,19); JNI_SLOT(PopLocalFrame,20); JNI_SLOT(NewGlobalRef,21);
JNI_SLOT(DeleteGlobalRef,22); JNI_SLOT(DeleteLocalRef,23); JNI_SLOT(NewObjectA,30);
JNI_SLOT(GetObjectClass,31); JNI_SLOT(GetMethodID,33); JNI_SLOT(CallObjectMethodA,36);
JNI_SLOT(CallVoidMethodA,63); JNI_SLOT(GetStaticMethodID,113); JNI_SLOT(CallStaticObjectMethodA,116);
JNI_SLOT(NewString,163); JNI_SLOT(GetStringLength,164); JNI_SLOT(GetArrayLength,171);
JNI_SLOT(NewObjectArray,172); JNI_SLOT(GetObjectArrayElement,173); JNI_SLOT(SetObjectArrayElement,174);
JNI_SLOT(NewIntArray,179); JNI_SLOT(GetIntArrayRegion,203); JNI_SLOT(GetFloatArrayRegion,205);
JNI_SLOT(SetIntArrayRegion,211); JNI_SLOT(GetStringRegion,220); JNI_SLOT(ExceptionCheck,228);
_Static_assert(offsetof(struct JNIInvokeInterface,GetEnv)==6*sizeof(void*),"JavaVM GetEnv");
_Static_assert(offsetof(struct JNIInvokeInterface,AttachCurrentThreadAsDaemon)==7*sizeof(void*),"JavaVM attach");
_Static_assert(offsetof(struct JNIInvokeInterface,DetachCurrentThread)==5*sizeof(void*),"JavaVM detach");
_Static_assert(sizeof(jvalue)==8 && sizeof(AndroidBitmapInfo)==20 && offsetof(AndroidBitmapInfo,flags)==16,"JNI/bitmap layout");

extern void ANativeActivity_onCreate(ANativeActivity *, void *, size_t);
static JavaVM *vm;
static jclass probe;
static jobject context;
static char *directory;

typedef struct {
    int32_t width, height, stride, length;
    uint8_t *pixels;
    int32_t thread;
} ProbeBitmap;
_Static_assert(sizeof(ProbeBitmap)==32 && offsetof(ProbeBitmap,pixels)==16,"test bitmap ABI");

__attribute__((visibility("default"))) const char *probe_directory(void) { return directory; }
// 测试自行保留 NativeActivity 交付的 VM，不依赖生产宿主的私有导出符号。
__attribute__((visibility("default"))) JavaVM *probe_java_vm(void) { return vm; }
__attribute__((visibility("default"))) int32_t probe_reference(void) {
    JNIEnv *env=NULL;
    if((*vm)->GetEnv(vm,(void**)&env,JNI_VERSION_1_6)!=JNI_OK)return -1;
    if((*env)->PushLocalFrame(env,8)<0)return -1;
    jmethodID method=(*env)->GetStaticMethodID(env,probe,"reference","(Ljava/lang/String;)V");
    jstring path=(*env)->NewStringUTF(env,directory);
    if(method)(*env)->CallStaticVoidMethod(env,probe,method,path);
    int failed=(*env)->ExceptionCheck(env);
    if(failed){(*env)->ExceptionDescribe(env);(*env)->ExceptionClear(env);}
    (*env)->PopLocalFrame(env,NULL);return failed?-1:0;
}
__attribute__((visibility("default"))) void probe_free(ProbeBitmap *b) { if(b){free(b->pixels);free(b);} }
__attribute__((visibility("default"))) ProbeBitmap *probe_run(void) {
    JNIEnv *env=NULL; int attached=0;
    jint result=(*vm)->GetEnv(vm,(void**)&env,JNI_VERSION_1_6);
    if(result==JNI_EDETACHED){if((*vm)->AttachCurrentThread(vm,&env,NULL)!=JNI_OK)return NULL;attached=1;}
    else if(result!=JNI_OK)return NULL;
    ProbeBitmap *out=NULL;
    if((*env)->PushLocalFrame(env,16)<0)goto done;
    jmethodID run=(*env)->GetStaticMethodID(env,probe,"run","(Landroid/content/Context;Ljava/lang/String;)[B");
    jstring path=(*env)->NewStringUTF(env,directory);
    jbyteArray bytes=run?(jbyteArray)(*env)->CallStaticObjectMethod(env,probe,run,context,path):NULL;
    if((*env)->ExceptionCheck(env)){(*env)->ExceptionDescribe(env);(*env)->ExceptionClear(env);goto pop;}
    if(!bytes || (*env)->GetArrayLength(env,bytes)!=800*1100*4)goto pop;
    out=calloc(1,sizeof(*out));if(!out)goto pop;
    out->width=800;out->height=1100;out->stride=3200;out->length=800*1100*4;out->thread=gettid();
    out->pixels=malloc(out->length);if(!out->pixels){free(out);out=NULL;goto pop;}
    (*env)->GetByteArrayRegion(env,bytes,0,out->length,(jbyte*)out->pixels);
    if((*env)->ExceptionCheck(env)){(*env)->ExceptionDescribe(env);(*env)->ExceptionClear(env);probe_free(out);out=NULL;}
pop: (*env)->PopLocalFrame(env,NULL);
done: if(attached)(*vm)->DetachCurrentThread(vm);return out;
}

__attribute__((visibility("default"))) void probe_onCreate(ANativeActivity *a,void *saved,size_t size) {
    // 引用归测试进程所有，保留至进程结束；不保留 Activity 或其 JNIEnv。
    if(!probe){
        JNIEnv *env=a->env;vm=a->vm;directory=strdup(a->internalDataPath);
        jclass activityClass=(*env)->GetObjectClass(env,a->clazz);
        jmethodID getContext=(*env)->GetMethodID(env,activityClass,"getApplicationContext","()Landroid/content/Context;");
        jobject app=(*env)->CallObjectMethod(env,a->clazz,getContext);context=(*env)->NewGlobalRef(env,app);
        jmethodID getLoader=(*env)->GetMethodID(env,activityClass,"getClassLoader","()Ljava/lang/ClassLoader;");
        jobject loader=(*env)->CallObjectMethod(env,a->clazz,getLoader);
        jclass loaderClass=(*env)->GetObjectClass(env,loader);
        jmethodID load=(*env)->GetMethodID(env,loaderClass,"loadClass","(Ljava/lang/String;)Ljava/lang/Class;");
        jstring name=(*env)->NewStringUTF(env,"org.golanggui.androidtext.TextProbe");
        jobject cls=(*env)->CallObjectMethod(env,loader,load,name);
        if((*env)->ExceptionCheck(env) || !cls || !context || !directory){(*env)->ExceptionDescribe(env);abort();}
        probe=(*env)->NewGlobalRef(env,cls);
        (*env)->DeleteLocalRef(env,cls);(*env)->DeleteLocalRef(env,name);(*env)->DeleteLocalRef(env,loaderClass);
        (*env)->DeleteLocalRef(env,loader);(*env)->DeleteLocalRef(env,app);(*env)->DeleteLocalRef(env,activityClass);
    }
    ANativeActivity_onCreate(a,saved,size);
}
`,
	})
}
