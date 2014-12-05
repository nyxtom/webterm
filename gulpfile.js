var browserSync = require("browser-sync");
var gulp = require("gulp");
var reload = browserSync.reload;

// watch files for changes
gulp.task("serve", function() {
    browserSync({
        server: {
            baseDir: "app"
        }
    });

    gulp.watch(["*.html", "styles/**/*.css", "scripts/**/*.js"], {cwd: "app"}, reload);
});
