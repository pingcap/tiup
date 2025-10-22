// get partitions info of the system

package insight

import (
	"os"
	"path"
	"strconv"
	"strings"

	si "github.com/AstroProfundis/sysinfo"
)

// BlockDev is similar to blkDev_cxt in lsblk (from util-linux)
// contains metadata of a block device
type BlockDev struct {
	Name       string     `json:"name,omitempty"`
	Partition  bool       `json:"partition,omitempty"`
	Mount      MountInfo  `json:"mount"`
	UUID       string     `json:"uuid,omitempty"`
	Sectors    uint64     `json:"sectors,omitempty"`
	Size       uint64     `json:"size,omitempty"`
	SubDev     []BlockDev `json:"subdev,omitempty"`
	Holder     []string   `json:"holder_of,omitempty"`
	Slave      []string   `json:"slave_of,omitempty"`
	Rotational string     `json:"rotational,omitempty"`
}

// MountInfo is the metadata of a mounted device
type MountInfo struct {
	MountPoint string `json:"mount_point,omitempty"`
	FSType     string `json:"filesystem,omitempty"`
	// Mount options used to mount this device
	Options string `json:"mount_options,omitempty"`
}

const (
	sysBlockPath  = "/sys/block"
	devMapperPath = "/dev/mapper"
)

// GetPartitionStats is getting disk partition statistics
func GetPartitionStats() []BlockDev {
	partStats := make([]BlockDev, 0)
	if dirSysBlk, err := os.Lstat(sysBlockPath); err == nil &&
		dirSysBlk.IsDir() {
		fi, _ := os.Open(sysBlockPath)
		blockDevs, _ := fi.Readdir(0)
		for _, blk := range blockDevs {
			var blkDev BlockDev
			if blkDev.getBlockDevice(blk, nil) {
				partStats = append(partStats, blkDev)
			}
		}
		matchUUIDs(partStats, getUUIDs())
		matchMounts(partStats, checkMounts())
	}
	return partStats
}

func (blkDev *BlockDev) getBlockDevice(blk os.FileInfo, parent os.FileInfo) bool {
	var fullpath string
	var dev string
	if parent != nil {
		fullpath = path.Join(sysBlockPath, parent.Name(), blk.Name())
		dev = fullpath
	} else {
		fullpath = path.Join(sysBlockPath, blk.Name())
		dev, _ = os.Readlink(fullpath)
	}

	if strings.HasPrefix(dev, "../devices/virtual/") &&
		(strings.Contains(dev, "ram") ||
			strings.Contains(dev, "loop")) {
		return false
	}

	// open the dir
	var fi *os.File
	if parent != nil {
		fi, _ = os.Open(dev)
	} else {
		fi, _ = os.Open(path.Join(sysBlockPath, dev))
	}
	subFiles, err := fi.Readdir(0)
	if err != nil {
		return false
	}

	// check for sub devices
	for _, subFile := range subFiles {
		// check if this is a partition
		if subFile.Name() == "partition" {
			blkDev.Partition = true
		}

		// populate subdev
		if strings.HasPrefix(subFile.Name(), blk.Name()) {
			var subBlk BlockDev
			subBlk.getBlockDevice(subFile, blk)
			blkDev.SubDev = append(blkDev.SubDev, subBlk)
		}
	}

	blkDev.Name = blk.Name()
	blkSec, err := strconv.Atoi(si.SlurpFile(path.Join(fullpath, "size")))
	if err == nil {
		blkDev.Sectors = uint64(blkSec)
		blkDev.Size = blkDev.Sectors << 9
	}

	slaves, holders := listDeps(blk.Name())
	if len(slaves) > 0 {
		for _, slave := range slaves {
			blkDev.Slave = append(blkDev.Slave, slave.Name())
		}
	}
	if len(holders) > 0 {
		for _, holder := range holders {
			blkDev.Holder = append(blkDev.Holder, holder.Name())
		}
	}

	blkDev.Rotational = si.SlurpFile(path.Join(fullpath, "queue/rotational"))

	return true
}

// listDeps check and return the dependency relationship of partitions
func listDeps(blk string) ([]os.FileInfo, []os.FileInfo) {
	fiSlaves, _ := os.Open(path.Join(sysBlockPath, blk, "slaves"))
	fileInfoHolders, _ := os.Open(path.Join(sysBlockPath, blk, "holders"))
	slaves, _ := fiSlaves.Readdir(0)
	holders, _ := fileInfoHolders.Readdir(0)
	return slaves, holders
}

// getUUIDs get UUIDs for partitions and put them in a map to device names
func getUUIDs() map[string]string {
	sysDiskUUID := "/dev/disk/by-uuid"
	fi, err := os.Open(sysDiskUUID)
	if err != nil {
		return nil
	}
	links, err := fi.Readdir(0)
	if err != nil {
		return nil
	}
	diskByUUID := make(map[string]string)
	for _, link := range links {
		if link.IsDir() {
			continue
		}
		blk, err := os.Readlink(path.Join(sysDiskUUID, link.Name()))
		if err != nil {
			continue
		}
		blkName := strings.TrimPrefix(blk, "../../")
		diskByUUID[blkName] = link.Name()
	}
	return diskByUUID
}

// matchUUIDs pair UUIDs and their other device information by names
func matchUUIDs(devs []BlockDev, diskByUUID map[string]string) {
	if len(devs) < 1 || diskByUUID == nil {
		return
	}

	// match devs to their UUIDs
	for i := range devs {
		devs[i].UUID = diskByUUID[devs[i].Name]

		// sub devices
		if len(devs[i].SubDev) < 1 {
			continue
		}
		matchUUIDs(devs[i].SubDev, diskByUUID)
	}
}

// checkMounts get meta info of mount points and put them in a map to device names
func checkMounts() map[string]MountInfo {
	raw, err := os.ReadFile(GetProcPath("mounts"))
	if err != nil {
		return nil
	}
	rawLines := strings.Split(string(raw), "\n")
	mountPoints := make(map[string]MountInfo)

	for _, line := range rawLines {
		mountInfo := strings.Split(line, " ")
		if len(mountInfo) < 6 {
			continue
		}
		var mp MountInfo
		mp.MountPoint = mountInfo[1]
		mp.FSType = mountInfo[2]
		mp.Options = mountInfo[3]
		devPath := strings.Split(mountInfo[0], "/")
		if len(devPath) < 1 {
			continue
		}
		devName := devPath[len(devPath)-1:][0]
		mountPoints[devName] = mp
	}

	// check for swap partitions
	// note: swap file is not supported yet, as virtual block devices
	// are excluded from final result
	if swaps, err := os.ReadFile(GetProcPath("swaps")); err == nil {
		swapLines := strings.Split(string(swaps), "\n")
		for i, line := range swapLines {
			// skip table headers and empty line
			if i == 0 ||
				line == "" {
				continue
			}
			devPath := strings.Split(strings.Fields(line)[0], "/")
			if len(devPath) < 1 {
				continue
			}
			var mp MountInfo
			mp.MountPoint = "[SWAP]"
			mp.FSType = "swap"
			devName := devPath[len(devPath)-1:][0]
			mountPoints[devName] = mp
		}
	}

	return mountPoints
}

// matchMounts pair mount point meta and their other device information by names
func matchMounts(devs []BlockDev, mountPoints map[string]MountInfo) {
	if len(devs) < 1 || mountPoints == nil {
		return
	}

	// read device mapper info
	// we build results by block device names, but the names in mount info file
	// are device mapper names, so we need to find the mapping list of them
	// errors are ignored when reading the dir
	devMapperNames := make(map[string]string)
	if dirDevMapper, err := os.Lstat(devMapperPath); err == nil && dirDevMapper.IsDir() {
		fi, _ := os.Open(devMapperPath)
		devMappers, _ := fi.Readdir(0)
		for _, mapper := range devMappers {
			fullpath := path.Join(devMapperPath, mapper.Name())
			dev, _ := os.Readlink(fullpath)

			devMapperNames[path.Base(dev)] = mapper.Name()
		}
	}

	for i := range devs {
		// find mount point info of mapped devices
		devName := devs[i].Name
		if mapperName, ok := devMapperNames[devName]; ok {
			devName = mapperName
		}
		devs[i].Mount = mountPoints[devName]

		// sub devices
		if len(devs[i].SubDev) < 1 {
			continue
		}
		matchMounts(devs[i].SubDev, mountPoints)
	}
}
