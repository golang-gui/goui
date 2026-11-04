package org.golanggui.androidtext;

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
}
