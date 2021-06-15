# Simple Photo Album

Port of my mod_perl album [Apache-Album](https://www.cpan.org/modules/by-module/Apache/Apache-Album-0.96.readme "Apache-Album")

## ABSTRACT
This is a simple photo album. Copy pngs/jpegs to a directory, create an optional text block (in a file called caption.txt) to go to the top, and the program does the rest.

Default settings are in config.yaml and may be overriden by using config.yaml files in directories with images.

## INSTALLATION

```
$ go get github.com/jddwoody/album
```

http://localhost:8000/

Displays a list of configured albums.

## CONFIGURATION

The configuration can be a little tricky, so here is a little more information. It's important to realize that there are two separate, but related directories. One is where the physical pictures reside, the other is where the "virtual" albums reside.Consider a filesystem called /albums exists and it is this filesystem that will house the images. Also consider that multiple people will have albums there, so you would create a directory for each user:

```
/albums/test/albums_loc
/albums/jdw/albums_loc
/albums/travis/albums_loc
```

Then in your config.yaml file add an entry for each directory. The minimum properties that must be set are albumTitle, albumDir and thumbDir:

```
albums:
  test:
    albumTitle: Test Album
    albumDir: "/albums/test/albums_loc"
    thumbDir: "/albums/test/thumbs"
```

The URL will be http://localhost:8000/test/albums/

This is the default [config.yaml](https://github.com/jddwoody/album/blob/main/resources/config.yaml "Default Config File") file

```
port: 8000
bodyArgs: bgcolor="#003939" rgb="#000000" text="#FFFFFF" link="#00ff00" vlink="#83BCBC"
default:
    bodyArgs: bgcolor="#003939" rgb="#000000" text="#FFFFFF" link="#00ff00" vlink="#83BCBC"
    thumbNailUse: width
    thumbNailWidth: 200
    thumbNailAspect: 1/5
    defaultBrowserWidth: 800
    slideShowDelay: 10
    outsideTableBorder: 1
    editMode: false
    allowFinalResize: false
    reverseDirs: false
    reversePics: false
albums:
  test:
    albumTitle: Test Album
    albumDir: "/tools/albums/albums_loc"
    thumbDir: "/tools/albums/thumbs"
```
### Directory Structure

Generally directories are sorted and have the same beautify as the caption files, ie a directory called Christmas_Party will have a link with the text "Christmas Party". An exception is that directories in the form:

.
+--01(January)
|  +--New_Years_Party
+--02(February)
|  +--Valetines_Day

Will sort by the intial numbers, but will only display "January" and "February"

```
January
    New Years Party
February
    Valentines Day
```

### caption.txt

A caption.txt file goes in a directory with images 

The caption.txt file consists of two parts. The first part is text/html that will be placed at the top of the html document. The second part is a mapping of filenames to captions. The module will do some simple mangling of the image file names to create the caption. But if it finds a mapping in the caption.txt file, that value is used instead. The value __END__ signifies the end of the first section and the beginning of the second.

For example:

|Filename|Bob_and_Jenny.jpg|
|Default Caption|Bob and Jenny|

caption.txt:
```
Bob_and_Jenny.jpg: This is me with my sister <EM>Jenny</EM>.
```

Here is a sample caption.txt file:

```
<H1>My Birthday Party</H1>

<center>This is me at my Birthday Party!.</center>

__END__
pieinface.gif: Here's me getting hit the face with a pie.
john5.jpg: This is <A HREF="mailto:johndoe@nowhere.com">John</A>
```




