package org.golanggui.androidtext;

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
