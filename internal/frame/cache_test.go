package frame

import (
	"bytes"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportCachedFrameCollectionShouldExportAndImport(t *testing.T) {
	cases := []int{
		1, 2, 3,
		11, 22, 33,
		1111, 2222, 3333,
		11111, 22222, 33333,
	}

	var (
		file     *bytes.Buffer = &bytes.Buffer{}
		filePeek *bytes.Buffer = &bytes.Buffer{}
	)

	for _, c := range cases {
		file.Reset()
		filePeek.Reset()

		var (
			collection       FrameCollection = mockFrameCollection(c)
			checksum         string          = "abcdef12345678900987654321abcdef12345678"
			frames           []*Frame        = collection.GetAll()
			importCollection FrameCollection
			importChecksum   string
			importFrames     []*Frame
		)

		err := ExportCachedFrameCollection(file, collection, checksum)
		assert.Nil(t, err)

		if _, err := filePeek.Write(file.Bytes()); err != nil {
			panic(err)
		}

		checksumEqual, err := ChecksumEqualPeek(filePeek, checksum)
		assert.Nil(t, err)
		assert.True(t, checksumEqual)

		importCollection, importChecksum, err = ImportCachedFrameCollection(file)
		assert.Nil(t, err)
		assert.Equal(t, checksum, importChecksum)
		assert.Equal(t, collection.Count(), importCollection.Count())

		importFrames = importCollection.GetAll()

		for index := 0; index < c; index += 1 {
			var (
				frame       *Frame = frames[index]
				importFrame *Frame = importFrames[index]
			)

			assert.Equal(t, frame.OrdinalNumber, importFrame.OrdinalNumber)
			assert.Equal(t, frame.Brightness, importFrame.Brightness)
			assert.Equal(t, frame.ColorDifference, importFrame.ColorDifference)
			assert.Equal(t, frame.BinaryThresholdDifference, importFrame.BinaryThresholdDifference)
			assert.Equal(t, frame.Timestamp, importFrame.Timestamp)
		}
	}
}

func TestCachedFrameCollectionShouldRoundTripTimestamps(t *testing.T) {
	const capacity = 6

	var (
		collection = NewFrameCollection(capacity)
		factory    = CreateFrameFactory(BinaryThresholdParam)
		img        = mockImage(color.White)
	)

	for index := 0; index < capacity; index += 1 {
		f, err := factory.CreateNewFrame(img, img, float64(index)*0.25)
		assert.Nil(t, err)
		assert.Nil(t, collection.Push(f))
	}

	buffer := &bytes.Buffer{}
	assert.Nil(t, ExportCachedFrameCollection(buffer, collection, "abcdef12345678900987654321abcdef12345678"))

	imported, _, err := ImportCachedFrameCollection(buffer)
	assert.Nil(t, err)
	assert.Equal(t, collection.Count(), imported.Count())

	for index, importedFrame := range imported.GetAll() {
		assert.InDelta(t, float64(index)*0.25, importedFrame.Timestamp, 1e-12)
	}
}

func TestChecksumEqualPeekShouldRejectPreviousFormatVersion(t *testing.T) {
	// A version-1 cache header must not be accepted by the version-2 reader, forcing a
	// graceful fallback to a fresh frame analysis.
	previousVersionHeader := append(append([]uint8{}, magicSequence...), 0x01)
	previousVersionHeader = append(previousVersionHeader, magicSequence...)

	equal, err := ChecksumEqualPeek(bytes.NewReader(previousVersionHeader), "abcdef12345678900987654321abcdef12345678")
	assert.Nil(t, err)
	assert.False(t, equal)
}

func mockFrameCollection(capacity int) FrameCollection {
	var (
		fc      = NewFrameCollection(capacity)
		factory = CreateFrameFactory(BinaryThresholdParam)
		img     = mockImage(color.White)
	)

	for index := 0; index < capacity; index += 1 {
		var (
			f   *Frame
			err error
		)

		if index == 0 {
			f, err = factory.CreateNewFrame(img, nil, 0)
		} else {
			f, err = factory.CreateNewFrame(img, img, 0)
		}

		if err != nil {
			panic("failed to create a frame for the mock frame collection")
		}

		if err = fc.Push(f); err != nil {
			panic("failed to push the frame to the mock frame collection")
		}
	}

	return fc
}
