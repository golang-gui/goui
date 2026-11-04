package org.golanggui.androidtext;

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
