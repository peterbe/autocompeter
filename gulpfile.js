// Include gulp
var gulp = require('gulp');

// Include Our Plugins
var jshint = require('gulp-jshint');
var sass = require('gulp-sass');
var concat = require('gulp-concat');
var uglify = require('gulp-uglify');
var rename = require('gulp-rename');
var cleanCSS = require('gulp-clean-css');
var header = require('gulp-header');

var bowerPkg = require('./bower.json');

var banner = '/* Autocompeter.com <%= pkg.version %> */\n';

// Lint Task
gulp.task('lint', function() {
    return gulp.src('src/*.js')
        .pipe(jshint())
        .pipe(jshint.reporter('default'));
});

// Compile Our Sass
gulp.task('sass', function() {
    return gulp.src('src/*.scss')
        .pipe(sass())
        .pipe(gulp.dest('public/dist'))
        .pipe(rename('autocompeter.min.css'))
        .pipe(cleanCSS({keepBreaks:true}))
        .pipe(header(banner, {pkg: bowerPkg}))
        .pipe(gulp.dest('public/dist'));
});

// Concatenate & Minify JS
gulp.task('scripts', function() {
    return gulp.src('src/*.js')
        .pipe(concat('autocompeter.js'))
        .pipe(gulp.dest('public/dist'))
        .pipe(rename('autocompeter.min.js'))
        .pipe(uglify())
        .pipe(header(banner, {pkg: bowerPkg}))
        .pipe(gulp.dest('public/dist'));
});

// Watch Files For Changes
gulp.task('watch', function() {
    gulp.watch('src/*.js', ['lint', 'scripts']);
    gulp.watch('src/*.scss', ['sass']);
});

// Default Task
gulp.task('default', ['lint', 'sass', 'scripts', 'watch']);
gulp.task('build', ['lint', 'sass', 'scripts']);
